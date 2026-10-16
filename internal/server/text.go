package server

import (
	"bytes"
	"html/template"
	"io"
	"net/http"
	"strconv"

	"fairdrop/internal/transfer"
)

var textPreviewTemplate = template.Must(template.New("text-preview").Parse(`<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Read text with FairDrop</title><style>:root{color-scheme:light dark;font-family:system-ui,sans-serif}body{max-width:40rem;margin:2rem auto;padding:1rem;line-height:1.5}button{font:inherit;min-height:44px;padding:.5rem 1rem}button:focus{outline:3px solid Highlight;outline-offset:3px}</style></head><body><main><h1>Text is ready</h1><p>{{.Length}} bytes of text. The first device to read it gets this text.</p><form method="post" action=""><button type="submit">Read text</button></form><p>Use only on a local network you trust.</p></main></body></html>`))
var textRevealTemplate = template.Must(template.New("text-reveal").Parse(`<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Text from FairDrop</title><style>:root{color-scheme:light dark;font-family:system-ui,sans-serif}body{max-width:40rem;margin:2rem auto;padding:1rem;line-height:1.5}textarea{box-sizing:border-box;width:100%;min-height:15rem;font:inherit;resize:vertical}textarea:focus{outline:3px solid Highlight;outline-offset:3px}</style></head><body><main><h1>Text from FairDrop</h1><label for="shared-text">Shared text</label><textarea id="shared-text" readonly spellcheck="false">
{{.}}</textarea><p>Select the text and use your browser or device’s Copy command.</p></main></body></html>`))

func (r *run) textRoute(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet && request.Method != http.MethodPost || !tokenMatches(request.PathValue("token"), r.token) {
		writeStatus(writer, http.StatusNotFound)
		return
	}
	if r.claimed.Load() {
		writeStatus(writer, http.StatusLocked)
		return
	}
	if request.Method == http.MethodGet {
		if r.ctx.Err() != nil || request.Context().Err() != nil {
			writeStatus(writer, http.StatusNotFound)
			return
		}
		var body bytes.Buffer
		if textPreviewTemplate.Execute(&body, struct{ Length int }{len(*r.text)}) != nil {
			writeStatus(writer, http.StatusInternalServerError)
			return
		}
		if r.ctx.Err() != nil || request.Context().Err() != nil {
			writeStatus(writer, http.StatusNotFound)
			return
		}
		setReceiverPageHeaders(writer.Header())
		writer.Header().Set("Content-Length", strconv.Itoa(body.Len()))
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write(body.Bytes())
		return
	}
	if !r.claimed.CompareAndSwap(false, true) {
		writeStatus(writer, http.StatusLocked)
		return
	}
	if err := r.authorizer.AuthorizeClaim(r.ctx, r.sessionID); err != nil {
		writeStatus(writer, http.StatusNotFound)
		return
	}
	if r.ctx.Err() != nil {
		writeStatus(writer, http.StatusNotFound)
		return
	}
	if request.Context().Err() != nil {
		r.finishTextDisconnect(writer)
		return
	}
	var body bytes.Buffer
	if err := textRevealTemplate.Execute(&body, *r.text); err != nil || body.Len() > 6*transfer.MaxTextBytes+16*1024 {
		writeStatus(writer, http.StatusGone)
		r.finalizeAfterResponse(request, failedEvent(r.sessionID, transfer.ProgressSnapshot{}, transfer.NewError(transfer.ErrTransferFailed, "text response could not be rendered")))
		return
	}
	if r.ctx.Err() != nil {
		writeStatus(writer, http.StatusNotFound)
		return
	}
	if request.Context().Err() != nil {
		r.finishTextDisconnect(writer)
		return
	}
	setReceiverPageHeaders(writer.Header())
	writer.Header().Set("Content-Length", strconv.Itoa(body.Len()))
	writer.WriteHeader(http.StatusOK)
	if flusher, ok := writer.(http.Flusher); ok {
		flusher.Flush()
	}
	progress := newMeter(int64(body.Len()), true, r.now, func(snapshot transfer.ProgressSnapshot) { r.lane.publishProgress(progressEvent(r.sessionID, snapshot)) })
	n, err := (&countingWriter{dst: writer, meter: progress}).Write(body.Bytes())
	if err == nil && n != body.Len() {
		err = io.ErrShortWrite
	}
	snapshot := progress.snapshot()
	if err == nil {
		r.finalizeAfterResponse(request, completeEvent(r.sessionID, snapshot))
		return
	}
	if isCancellation(err) {
		r.finish(nil)
	} else {
		event := failedEvent(r.sessionID, snapshot, err)
		r.finish(&event)
	}
	panic(http.ErrAbortHandler)
}

// A request may disappear just after the coordinator commits TRANSFERRING.
// That is a failed transfer, not a session Cancel; leaving it silent would
// strand the coordinator in an active state forever.
func (r *run) finishTextDisconnect(writer http.ResponseWriter) {
	writeStatus(writer, http.StatusNotFound)
	event := failedEvent(r.sessionID, transfer.ProgressSnapshot{}, transfer.NewError(transfer.ErrTransferFailed, "text receiver disconnected before reveal"))
	r.finish(&event)
}
