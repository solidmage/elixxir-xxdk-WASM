///////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 xx network SEZC                                          //
//                                                                           //
// Use of this source code is governed by a license that can be found in the //
// LICENSE file                                                              //
///////////////////////////////////////////////////////////////////////////////

//go:build js && wasm

package main

import (
	"fmt"
	"gitlab.com/elixxir/xxdk-wasm/wasm"
	"os"
	"syscall/js"
)

func main() {
	fmt.Println("Go Web Assembly")

	// wasm/cmix.go
	js.Global().Set("NewCmix", js.FuncOf(wasm.NewCmix))
	js.Global().Set("LoadCmix", js.FuncOf(wasm.LoadCmix))

	// wasm/e2e.go
	js.Global().Set("Login", js.FuncOf(wasm.Login))
	js.Global().Set("LoginEphemeral", js.FuncOf(wasm.LoginEphemeral))

	// wasm/identity.go
	js.Global().Set("StoreReceptionIdentity",
		js.FuncOf(wasm.StoreReceptionIdentity))
	js.Global().Set("LoadReceptionIdentity",
		js.FuncOf(wasm.LoadReceptionIdentity))
	js.Global().Set("GetIDFromContact",
		js.FuncOf(wasm.GetIDFromContact))
	js.Global().Set("GetPubkeyFromContact",
		js.FuncOf(wasm.GetPubkeyFromContact))
	js.Global().Set("SetFactsOnContact",
		js.FuncOf(wasm.SetFactsOnContact))
	js.Global().Set("GetFactsFromContact",
		js.FuncOf(wasm.GetFactsFromContact))

	// wasm/params.go
	js.Global().Set("GetDefaultCMixParams",
		js.FuncOf(wasm.GetDefaultCMixParams))
	js.Global().Set("GetDefaultE2EParams",
		js.FuncOf(wasm.GetDefaultE2EParams))
	js.Global().Set("GetDefaultFileTransferParams",
		js.FuncOf(wasm.GetDefaultFileTransferParams))
	js.Global().Set("GetDefaultSingleUseParams",
		js.FuncOf(wasm.GetDefaultSingleUseParams))
	js.Global().Set("GetDefaultE2eFileTransferParams",
		js.FuncOf(wasm.GetDefaultE2eFileTransferParams))

	// wasm/logging.go
	js.Global().Set("LogLevel", js.FuncOf(wasm.LogLevel))
	js.Global().Set("RegisterLogWriter", js.FuncOf(wasm.RegisterLogWriter))
	js.Global().Set("EnableGrpcLogs", js.FuncOf(wasm.EnableGrpcLogs))

	// wasm/ndf.go
	js.Global().Set("DownloadAndVerifySignedNdfWithUrl",
		js.FuncOf(wasm.DownloadAndVerifySignedNdfWithUrl))

	// wasm/version.go
	js.Global().Set("GetVersion", js.FuncOf(wasm.GetVersion))
	js.Global().Set("GetGitVersion", js.FuncOf(wasm.GetGitVersion))
	js.Global().Set("GetDependencies", js.FuncOf(wasm.GetDependencies))

	// wasm/secrets.go
	js.Global().Set("GenerateSecret", js.FuncOf(wasm.GenerateSecret))

	// wasm/dummy.go
	js.Global().Set("NewDummyTrafficManager",
		js.FuncOf(wasm.NewDummyTrafficManager))

	// bindings/broadcast.go
	js.Global().Set("NewBroadcastChannel", js.FuncOf(wasm.NewBroadcastChannel))

	// bindings/backup.go
	js.Global().Set("NewCmixFromBackup", js.FuncOf(wasm.NewCmixFromBackup))
	js.Global().Set("InitializeBackup", js.FuncOf(wasm.InitializeBackup))
	js.Global().Set("ResumeBackup", js.FuncOf(wasm.ResumeBackup))

	// bindings/errors.go
	js.Global().Set("CreateUserFriendlyErrorMessage",
		js.FuncOf(wasm.CreateUserFriendlyErrorMessage))
	js.Global().Set("UpdateCommonErrors",
		js.FuncOf(wasm.UpdateCommonErrors))

	<-make(chan bool)
	os.Exit(0)
}