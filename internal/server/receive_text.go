package server

import (
	"context"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"fairdrop/internal/transfer"
)

const receiveTextPattern = "/receive-text/{token}"
const maxReceiveTextFormBytes = 3*transfer.MaxRawTextBytes + 1024

const receiveTextForm = `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Send text with FairDrop</title><style>` + receiverCSS + `textarea{box-sizing:border-box;width:100%;min-height:12rem;font:inherit;resize:vertical}textarea:focus{outline:3px solid Highlight;outline-offset:3px}</style></head><body><main><div class="brand">FairDrop</div><h1>Send text to this computer</h1><p class="detail">Enter a note or link, up to 65,536 UTF-8 bytes.</p><form method="post" action="" enctype="application/x-www-form-urlencoded" accept-charset="UTF-8" autocomplete="off"><label for="text">Text to send</label><textarea id="text" name="text" required autocomplete="off"></textarea><button type="submit">Send text</button></form><p class="note">Use only on a local network you trust. The text stays on this computer until dismissed.</p></main></body></html>`
const receiveTextSuccess = `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Text received</title><style>` + receiverCSS + `</style></head><body><main><div class="brand">FairDrop</div><h1>Text received by this computer</h1><p class="detail">You can return to your computer to inspect and copy it.</p></main></body></html>`
const receiveTextInvalid = `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Text not accepted</title><style>` + receiverCSS + `</style></head><body><main><div class="brand">FairDrop</div><h1>Text not accepted</h1><p class="detail">Enter supported text up to 65,536 UTF-8 bytes and try again.</p><a href="">Try again</a></main></body></html>`
const receiveTextTooLarge = `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Text too large</title><style>` + receiverCSS + `</style></head><body><main><div class="brand">FairDrop</div><h1>Text too large</h1><p class="detail">Enter less text and try again.</p><a href="">Try again</a></main></body></html>`

func writeReceiveTextPage(w http.ResponseWriter, status int, body string) {
	setReceiverPageHeaders(w.Header())
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(status)
	_, _ = io.WriteString(w, body)
}

func (r *run) receiveTextRoute(w http.ResponseWriter, request *http.Request) {
	if (request.Method != http.MethodGet && request.Method != http.MethodPost) || !tokenMatches(request.PathValue("token"), r.token) {
		writeStatus(w, http.StatusNotFound)
		return
	}
	if r.claimed.Load() {
		writeStatus(w, http.StatusLocked)
		return
	}
	if request.Method == http.MethodGet {
		if request.Context().Err() != nil || r.ctx.Err() != nil {
			writeStatus(w, http.StatusNotFound)
			return
		}
		writeReceiveTextPage(w, http.StatusOK, receiveTextForm)
		return
	}
	if !r.processing.CompareAndSwap(false, true) {
		writeStatus(w, http.StatusLocked)
		return
	}
	defer r.processing.Store(false)
	if r.claimed.Load() || r.ctx.Err() != nil {
		writeStatus(w, http.StatusLocked)
		return
	}
	if request.ContentLength > maxReceiveTextFormBytes {
		writeReceiveTextPage(w, http.StatusRequestEntityTooLarge, receiveTextTooLarge)
		return
	}
	media, params, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if err != nil || media != "application/x-www-form-urlencoded" || len(params) > 1 || len(params) == 1 && !strings.EqualFold(params["charset"], "UTF-8") {
		writeReceiveTextPage(w, http.StatusBadRequest, receiveTextInvalid)
		return
	}
	body, err := io.ReadAll(io.LimitReader(request.Body, maxReceiveTextFormBytes+1))
	if len(body) > maxReceiveTextFormBytes {
		writeReceiveTextPage(w, http.StatusRequestEntityTooLarge, receiveTextTooLarge)
		return
	}
	if err != nil || request.Context().Err() != nil || r.ctx.Err() != nil ||
		request.ContentLength >= 0 && int64(len(body)) != request.ContentLength {
		writeReceiveTextPage(w, http.StatusBadRequest, receiveTextInvalid)
		return
	}
	fields, err := url.ParseQuery(string(body))
	if err != nil || len(fields) != 1 || len(fields["text"]) != 1 {
		writeReceiveTextPage(w, http.StatusBadRequest, receiveTextInvalid)
		return
	}
	value, err := transfer.NormalizeText(fields["text"][0])
	if err != nil {
		writeReceiveTextPage(w, http.StatusBadRequest, receiveTextInvalid)
		return
	}
	if request.Context().Err() != nil || r.ctx.Err() != nil {
		writeStatus(w, http.StatusNotFound)
		return
	}
	if !r.claimed.CompareAndSwap(false, true) {
		writeStatus(w, http.StatusLocked)
		return
	}
	// The caller's cancellation must be visible synchronously to commit. An
	// AfterFunc from the request leaves a window after AuthorizeClaim returns.
	claimCtx, stopClaim := context.WithCancel(request.Context())
	defer stopClaim()
	stopRunWatch := context.AfterFunc(r.ctx, stopClaim)
	defer stopRunWatch()
	if request.Context().Err() != nil {
		r.claimed.Store(false)
		writeStatus(w, http.StatusNotFound)
		return
	}
	if err := r.authorizer.AuthorizeClaim(claimCtx, r.sessionID); err != nil {
		if request.Context().Err() != nil && r.ctx.Err() == nil {
			r.claimed.Store(false)
		}
		writeStatus(w, http.StatusNotFound)
		return
	}
	if request.Context().Err() != nil {
		writeStatus(w, http.StatusNotFound)
		if r.ctx.Err() == nil {
			event := failedEvent(r.sessionID, transfer.ProgressSnapshot{}, transfer.NewError(transfer.ErrTransferFailed, "text receiver disconnected before commit"))
			r.finish(&event)
		}
		return
	}
	if err := r.authorizer.(transfer.ReceiveTextCommitter).CommitReceivedText(claimCtx, r.sessionID, value); err != nil {
		writeStatus(w, http.StatusNotFound)
		if r.ctx.Err() == nil {
			event := failedEvent(r.sessionID, transfer.ProgressSnapshot{}, transfer.NewError(transfer.ErrTransferFailed, "text could not be received"))
			r.finish(&event)
		}
		return
	}
	writeReceiveTextPage(w, http.StatusOK, receiveTextSuccess)
	r.finalizeAfterResponse(request, completeEvent(r.sessionID, transfer.ProgressSnapshot{}))
}
