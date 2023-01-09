////////////////////////////////////////////////////////////////////////////////
// Copyright © 2022 xx foundation                                             //
//                                                                            //
// Use of this source code is governed by a license that can be found in the  //
// LICENSE file.                                                              //
////////////////////////////////////////////////////////////////////////////////

//go:build js && wasm

package indexedDbWorker

import (
	"encoding/json"
	"github.com/pkg/errors"
	jww "github.com/spf13/jwalterweatherman"
	"gitlab.com/elixxir/xxdk-wasm/utils"
	"sync"
	"syscall/js"
	"time"
)

// TODO:
//  1. fix tag system
//  2. restructure packages
//  3. Get path to JS file from bindings
//  4. Add tests for worker.go and messageHandler.go

// InitID is the ID for the first item in the handler list. If the list only
// contains one handler, then this is the ID of that handler. If the list has
// autogenerated unique IDs, this is the initial ID to start at.
const InitID = uint64(0)

// Response timeouts.
const (
	// workerInitialConnectionTimeout is the time to wait to receive initial
	// contact from a new worker before timing out.
	workerInitialConnectionTimeout = 16 * time.Second

	// ResponseTimeout is the general time to wait after sending a message to
	// receive a response before timing out.
	ResponseTimeout = 8 * time.Second
)

// HandlerFn is the function that handles incoming data from the worker.
type HandlerFn func(data []byte)

// WorkerHandler manages the handling of messages received from the worker.
type WorkerHandler struct {
	// worker is the Worker Javascript object.
	// Doc: https://developer.mozilla.org/en-US/docs/Web/API/Worker
	worker js.Value

	// handlers are a list of handlers that handle a specific message received
	// from the worker. Each handler is keyed on a tag specifying how the
	// received message should be handled. If the message is a reply to a
	// message sent to the worker, then the handler is also keyed on a unique
	// ID. If the message is not a reply, then it appears on InitID.
	handlers map[Tag]map[uint64]HandlerFn

	// handlerIDs is a list of the newest ID to assign to each handler when
	// registered. The IDs are used to connect a reply from the worker to the
	// original message sent by the main thread.
	handlerIDs map[Tag]uint64

	// name describes the worker. It is used for debugging and logging purposes.
	name string

	mux sync.Mutex
}

// WorkerMessage is the outer message that contains the contents of each message
// sent to the worker. It is transmitted as JSON.
type WorkerMessage struct {
	Tag  Tag    `json:"tag"`
	ID   uint64 `json:"id"`
	Data []byte `json:"data"`
}

// NewWorkerHandler generates a new WorkerHandler. This functions will only
// return once communication with the worker has been established.
func NewWorkerHandler(aURL, name string) (*WorkerHandler, error) {
	// Create new worker options with the given name
	opts := newWorkerOptions("", "", name)

	wh := &WorkerHandler{
		worker:     js.Global().Get("Worker").New(aURL, opts),
		handlers:   make(map[Tag]map[uint64]HandlerFn),
		handlerIDs: make(map[Tag]uint64),
		name:       name,
	}

	// Register listeners on the Javascript worker object that receive messages
	// and errors from the worker
	wh.addEventListeners()

	// Register a handler that will receive initial message from worker
	// indicating that it is ready
	ready := make(chan struct{})
	wh.RegisterHandler(
		ReadyTag, InitID, false, func([]byte) { ready <- struct{}{} })

	// Wait for the ready signal from the worker
	select {
	case <-ready:
	case <-time.After(workerInitialConnectionTimeout):
		return nil, errors.Errorf("[WW] [%s] timed out after %s waiting for "+
			"initial message from worker",
			wh.name, workerInitialConnectionTimeout)
	}

	return wh, nil
}

// SendMessage sends a message to the worker with the given tag. If a reception
// handler is specified, then the message is given a unique ID to handle the
// reply. Set receptionHandler to nil if no reply is expected.
func (wh *WorkerHandler) SendMessage(
	tag Tag, data []byte, receptionHandler HandlerFn) {
	var id uint64
	if receptionHandler != nil {
		id = wh.RegisterHandler(tag, 0, true, receptionHandler)
	}

	jww.DEBUG.Printf("[WW] [%s] Main sending message for %q and ID %d with "+
		"data: %s", wh.name, tag, id, data)

	msg := WorkerMessage{
		Tag:  tag,
		ID:   id,
		Data: data,
	}
	payload, err := json.Marshal(msg)
	if err != nil {
		jww.FATAL.Panicf("[WW] [%s] Main failed to marshal %T for %q and "+
			"ID %d going to worker: %+v", wh.name, msg, tag, id, err)
	}

	go wh.postMessage(string(payload))
}

// receiveMessage is registered with the Javascript event listener and is called
// every time a new message from the worker is received.
func (wh *WorkerHandler) receiveMessage(data []byte) error {
	var msg WorkerMessage
	err := json.Unmarshal(data, &msg)
	if err != nil {
		return err
	}
	jww.DEBUG.Printf("[WW] [%s] Main received message for %q and ID %d with "+
		"data: %s", wh.name, msg.Tag, msg.ID, msg.Data)

	handler, err := wh.getHandler(msg.Tag, msg.ID)
	if err != nil {
		return err
	}

	go handler(msg.Data)

	return nil
}

// getHandler returns the handler with the given ID or returns an error if no
// handler is found. The handler is deleted from the map if specified in
// deleteAfterReceiving. This function is thread safe.
func (wh *WorkerHandler) getHandler(tag Tag, id uint64) (HandlerFn, error) {
	wh.mux.Lock()
	defer wh.mux.Unlock()
	handlers, exists := wh.handlers[tag]
	if !exists {
		return nil, errors.Errorf("no handlers found for tag %q", tag)
	}

	handler, exists := handlers[id]
	if !exists {
		return nil, errors.Errorf("no %q handler found for ID %d", tag, id)
	}

	if _, exists = deleteAfterReceiving[tag]; exists {
		delete(wh.handlers[tag], id)
		if len(wh.handlers[tag]) == 0 {
			delete(wh.handlers, tag)
		}
	}

	return handler, nil
}

// RegisterHandler registers the handler for the given tag and ID unless autoID
// is true, in which case a unique ID is used. Returns the ID that was
// registered. If a previous handler was registered, it is overwritten.
// This function is thread safe.
func (wh *WorkerHandler) RegisterHandler(
	tag Tag, id uint64, autoID bool, handler HandlerFn) uint64 {
	wh.mux.Lock()
	defer wh.mux.Unlock()

	if autoID {
		id = wh.getNextID(tag)
	}

	jww.DEBUG.Printf("[WW] [%s] Main registering handler for tag %q and ID %d "+
		"(autoID: %t)", wh.name, tag, id, autoID)

	if _, exists := wh.handlers[tag]; !exists {
		wh.handlers[tag] = make(map[uint64]HandlerFn)
	}
	wh.handlers[tag][id] = handler

	return id
}

// getNextID returns the next unique ID for the given tag. This function is not
// thread-safe.
func (wh *WorkerHandler) getNextID(tag Tag) uint64 {
	if _, exists := wh.handlerIDs[tag]; !exists {
		wh.handlerIDs[tag] = InitID
	}

	id := wh.handlerIDs[tag]
	wh.handlerIDs[tag]++
	return id
}

////////////////////////////////////////////////////////////////////////////////
// Javascript Call Wrappers                                                   //
////////////////////////////////////////////////////////////////////////////////

// addEventListeners adds the event listeners needed to receive a message from
// the worker. Two listeners were added; one to receive messages from the worker
// and the other to receive errors.
func (wh *WorkerHandler) addEventListeners() {
	// Create a listener for when the message event is fired on the worker. This
	// occurs when a message is received from the worker.
	// Doc: https://developer.mozilla.org/en-US/docs/Web/API/Worker/message_event
	messageEvent := js.FuncOf(func(_ js.Value, args []js.Value) any {
		err := wh.receiveMessage([]byte(args[0].Get("data").String()))
		if err != nil {
			jww.ERROR.Printf("[WW] [%s] Failed to receive message from "+
				"worker: %+v", wh.name, err)
		}
		return nil
	})

	// Create listener for when a messageerror event is fired on the worker.
	// This occurs when it receives a message that cannot be deserialized.
	// Doc: https://developer.mozilla.org/en-US/docs/Web/API/Worker/messageerror_event
	messageError := js.FuncOf(func(_ js.Value, args []js.Value) any {
		event := args[0]
		jww.ERROR.Printf("[WW] [%s] Main received error message from worker: %s",
			wh.name, utils.JsToJson(event))
		return nil
	})

	// Register each event listener on the worker using addEventListener
	// Doc: https://developer.mozilla.org/en-US/docs/Web/API/EventTarget/addEventListener
	wh.worker.Call("addEventListener", "message", messageEvent)
	wh.worker.Call("addEventListener", "messageerror", messageError)
}

// postMessage sends a message to the worker.
//
// message is the object to deliver to the worker; this will be in the data
// field in the event delivered to the worker. It must be a js.Value or a
// primitive type that can be converted via js.ValueOf. The Javascript object
// must be "any value or JavaScript object handled by the structured clone
// algorithm, which includes cyclical references.". See the doc for more
// information.
//
// If the message parameter is not provided, a SyntaxError will be thrown by the
// parser. If the data to be passed to the worker is unimportant, js.Null or
// js.Undefined can be passed explicitly.
//
// Doc: https://developer.mozilla.org/en-US/docs/Web/API/Worker/postMessage
func (wh *WorkerHandler) postMessage(msg any) {
	wh.worker.Call("postMessage", msg)
}

// Terminate immediately terminates the Worker. This does not offer the worker
// an opportunity to finish its operations; it is stopped at once.
//
// Doc: https://developer.mozilla.org/en-US/docs/Web/API/Worker/terminate
func (wh *WorkerHandler) Terminate() {
	wh.worker.Call("terminate")
}

// newWorkerOptions creates a new Javascript object containing optional
// properties that can be set when creating a new worker.
//
// Each property is optional; leave a property empty to use the defaults (as
// documented). The available properties are:
//   - type - The type of worker to create. The value can be either "classic" or
//     "module". If not specified, the default used is classic.
//   - credentials - The type of credentials to use for the worker. The value
//     can be "omit", "same-origin", or "include". If it is not specified, or if
//     the type is "classic", then the default used is "omit" (no credentials
//     are required).
//   - name - An identifying name for the worker, used mainly for debugging
//     purposes.
//
// Doc: https://developer.mozilla.org/en-US/docs/Web/API/Worker/Worker#options
func newWorkerOptions(workerType, credentials, name string) js.Value {
	options := make(map[string]any, 3)
	if workerType != "" {
		options["type"] = workerType
	}
	if credentials != "" {
		options["credentials"] = credentials
	}
	if name != "" {
		options["name"] = name
	}
	return js.ValueOf(options)
}
