////////////////////////////////////////////////////////////////////////////////
// Copyright © 2022 xx foundation                                             //
//                                                                            //
// Use of this source code is governed by a license that can be found in the  //
// LICENSE file.                                                              //
////////////////////////////////////////////////////////////////////////////////

//go:build js && wasm

package wasm

import (
	"gitlab.com/elixxir/client/bindings"
	"gitlab.com/elixxir/xxdk-wasm/utils"
	"syscall/js"
)

// DownloadAndVerifySignedNdfWithUrl retrieves the NDF from a specified URL.
// The NDF is processed into a protobuf containing a signature that is verified
// using the cert string passed in. The NDF is returned as marshaled byte data
// that may be used to start a client.
//
// Parameters:
//  - args[0] - The URL to download from (string).
//  - args[1] - The NDF certificate (string).
//
// Returns a promise:
//  - Resolves to the JSON of the NDF ([ndf.NetworkDefinition]) (Uint8Array).
//  - Rejected with an error if downloading fails.
func DownloadAndVerifySignedNdfWithUrl(_ js.Value, args []js.Value) interface{} {
	promiseFn := func(resolve, reject func(args ...interface{}) js.Value) {
		ndf, err := bindings.DownloadAndVerifySignedNdfWithUrl(
			args[0].String(), args[1].String())
		if err != nil {
			reject(utils.JsTrace(err))
		} else {
			resolve(utils.CopyBytesToJS(ndf))
		}
	}

	return utils.CreatePromise(promiseFn)
}
