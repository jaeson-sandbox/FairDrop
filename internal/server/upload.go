package server

import (
	"bytes"
	"context"
	"errors"
	"html/template"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"strconv"
	"time"

	"fairdrop/internal/transfer"
)

// This file is the receive half of the server: the one upload route, the
// script-free page that posts to it, and the streaming of a phone's multipart
// body into the session's destination.
//
// The route follows the download route's rules exactly. It is methodless so a
// wrong method looks like a wrong path; the token is matched in constant time;
// every rejection that is not about a request this token holder made is the same
// bare 404; and a claimed session answers 423 while its listener lives. What is
// new is only what a receive has to decide before it commits to a claim: a
// request that cannot possibly succeed (no declared length, not multipart, no
// file part, too large for the disk) is answered with a fixed page and leaves
// the session waiting, so a mistaken tap on the phone does not burn the one-time
// link.

// uploadPattern is the one route a receive run answers. Like downloadPattern it
// is methodless, and the token is a wildcard this package never parses itself.
const uploadPattern = "/upload/{token}"

const (
	// uploadField is the only form field whose file parts are written. Every
	// other part is read and discarded, within the declared size.
	uploadField = "files"

	// maxPartHeaderBytes bounds what the multipart reader may consume looking
	// for the next part's headers, so a hostile body cannot make a header the
	// size of the upload. Real part headers are a few hundred bytes.
	maxPartHeaderBytes = 32 << 10
)

// errBodyOverrun and errPartHeaders are the two ways the request body itself is
// refused. Neither text ever reaches a response.
var (
	errBodyOverrun = errors.New("server: the upload body is longer than its declared length")
	errPartHeaders = errors.New("server: a multipart part header is too large")
)

const receiverCSS = `:root{color-scheme:light dark;font-family:system-ui,-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;background:#F2F2F4;color:#1A1A1C}
*{box-sizing:border-box}body{margin:0;min-height:100vh;display:grid;grid-template-columns:minmax(0,1fr);place-items:center;padding:24px 16px}
main{min-width:0;width:min(100%,460px);overflow-wrap:anywhere;padding:clamp(24px,6vw,40px);border:1px solid #D6D6D6;border-radius:20px;background:#FFFFFF;box-shadow:0 12px 32px #1A1A1C12}
.brand{font-size:14px;font-weight:700;letter-spacing:.08em;text-transform:uppercase;color:#9C5636}
h1{font-size:clamp(26px,7vw,34px);line-height:1.15;margin:28px 0 8px}
.detail{color:#65656B;line-height:1.5;margin:12px 0 28px}
label{display:block;font-weight:650;margin:0 0 8px}
input[type=file]{font:inherit;width:100%;max-width:100%;margin:0 0 20px;padding:10px;border:1px solid #D6D6D6;border-radius:12px;background:#FFFFFF;color:inherit}
input[type=file]:focus-visible{outline:2px solid #9C5636;outline-offset:2px}
button{font:inherit;font-weight:700;min-height:48px;width:100%;border:0;border-radius:12px;background:#9C5636;color:#FFFFFF;cursor:pointer}
button:hover{background:#7F4428}button:focus-visible{outline:2px solid #9C5636;outline-offset:4px;box-shadow:0 0 0 2px #FFFFFF}
.note{font-size:14px;line-height:1.55;color:#65656B;margin:24px 0 0}
@media(prefers-color-scheme:dark){:root{background:#161618;color:#F2F2F4}main{background:#1F1F22;border-color:#47474A;box-shadow:none}.brand{color:#E39B70}.detail,.note{color:#9C9CA4}input[type=file]{background:#1F1F22;border-color:#47474A}input[type=file]:focus-visible{outline-color:#E39B70}button{background:#E39B70;color:#2B1206}button:hover{background:#F0B694}button:focus-visible{outline-color:#E39B70;box-shadow:0 0 0 2px #1F1F22}}
@media(forced-colors:active){main{border:1px solid CanvasText}button{border:1px solid ButtonText}button:focus-visible,input[type=file]:focus-visible{outline:3px solid Highlight}}`

// uploadTemplate renders the upload form and every fixed result page. The only
// dynamic values are copy this package chose and a saved-file count formatted
// here, and html/template escapes all of them anyway. Nothing the phone sent --
// no name, no size -- is ever echoed, so the result page has no way to disclose a
// path or to be made to say something the sender chose.
//
// The form's action is empty, which resolves to the current URL: a same-origin
// relative action that never comes from Host, a query, or the destination.
var uploadTemplate = template.Must(template.New("upload").Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>{{.Title}}</title><style>` + receiverCSS + `
</style></head><body><main><div class="brand">FairDrop</div><h1>{{.Heading}}</h1>
<p class="detail">{{.Detail}}</p>
{{if .Form}}<form method="post" action="" enctype="multipart/form-data"><label for="files">{{.Label}}</label><input id="files" type="file" name="files" multiple required><button type="submit">{{.Button}}</button></form>
{{end}}<p class="note">{{.Note}}</p>
</main></body></html>`))

// Receiver-owned copy for the upload pages, tabulated beside the download
// page's under Receiver page copy in the Quartz EXPERIENCE. The wording says
// "saved to this computer" only on a result page, after the files were renamed
// into place, and never claims more than the filesystem itself guarantees.
const (
	uploadTitle  = "Send files with FairDrop"
	uploadLabel  = "Files to send"
	uploadButton = "Upload"
	uploadNote   = "Use only on a local network you trust. FairDrop saves what you send to this computer, and accepts one upload."

	uploadHeading = "Send files to this computer"
	uploadDetail  = "Choose the files to send. They are saved to this computer in a new folder."

	uploadLengthHeading = "Upload not accepted"
	uploadLengthDetail  = "This browser did not say how large the upload is, so FairDrop cannot accept it. Try again from another browser."

	uploadEmptyHeading = "Nothing to upload"
	uploadEmptyDetail  = "No files were chosen. Choose at least one file, then upload."

	uploadLargeHeading = "Too large for this computer"
	uploadLargeDetail  = "This upload needs more space than this computer can spare. Choose fewer or smaller files and try again."

	uploadDoneHeading       = "Upload complete"
	uploadIncompleteHeading = "Upload incomplete"
)

type uploadPageData struct {
	Title   string
	Heading string
	Detail  string
	Label   string
	Button  string
	Note    string
	Form    bool
}

// formPage is a page that offers the form again: the upload page itself, and the
// refusals that leave the session waiting so the phone can try again.
func formPage(heading, detail string) uploadPageData {
	return uploadPageData{Title: uploadTitle, Heading: heading, Detail: detail, Label: uploadLabel, Button: uploadButton, Note: uploadNote, Form: true}
}

// resultPage is a terminal page: no form, because the session is over.
func resultPage(heading, detail string) uploadPageData {
	return uploadPageData{Title: uploadTitle, Heading: heading, Detail: detail, Note: uploadNote}
}

// savedCount spells the saved count the way both result pages state it. It is
// the only number either page contains.
func savedCount(saved int) string {
	if saved == 1 {
		return "1 file saved to this computer."
	}
	return strconv.Itoa(saved) + " files saved to this computer."
}

// writePage renders before it sends a header, so a template failure cannot
// masquerade as the status it was about to carry, then writes the page with the
// receiver page headers and an exact Content-Length.
func writePage(writer http.ResponseWriter, status int, data uploadPageData) {
	var body bytes.Buffer
	if err := uploadTemplate.Execute(&body, data); err != nil {
		writeStatus(writer, http.StatusInternalServerError)
		return
	}
	header := writer.Header()
	setReceiverPageHeaders(header)
	header.Set("Content-Length", strconv.Itoa(body.Len()))
	writer.WriteHeader(status)
	_, _ = writer.Write(body.Bytes())
}

// upload runs the receive route in the one order that keeps a wrong guess
// indistinguishable from a nonexistent resource and keeps the coordinator the
// only thing that can authorize a session.
func (r *run) upload(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet && request.Method != http.MethodPost {
		writeStatus(writer, http.StatusNotFound)
		return
	}
	if !tokenMatches(request.PathValue("token"), r.token) {
		writeStatus(writer, http.StatusNotFound)
		return
	}
	if r.claimed.Load() {
		writeStatus(writer, http.StatusLocked)
		return
	}
	if request.Method == http.MethodGet {
		r.uploadPage(writer, request)
		return
	}
	r.receive(writer, request)
}

// uploadPage serves the form. Like the landing page it is a preview-safe GET: it
// reserves nothing, authorizes nothing, reads no destination and publishes no
// event.
func (r *run) uploadPage(writer http.ResponseWriter, request *http.Request) {
	if request.Context().Err() != nil || r.ctx.Err() != nil {
		writeStatus(writer, http.StatusNotFound)
		return
	}
	writePage(writer, http.StatusOK, formPage(uploadHeading, uploadDetail))
}

// receive handles a POST. Everything before the claim writes nothing and leaves
// the session waiting; the claim happens only once a file part has been seen.
func (r *run) receive(writer http.ResponseWriter, request *http.Request) {
	// A declared length is the whole of the space check and of the progress
	// total, so a request without one cannot be accepted. A header-less POST
	// reads as length zero in net/http, which is not the same as "0": the header
	// itself is what distinguishes them.
	if request.ContentLength < 0 || (request.ContentLength == 0 && request.Header.Get("Content-Length") == "") {
		writePage(writer, http.StatusLengthRequired, formPage(uploadLengthHeading, uploadLengthDetail))
		return
	}
	declared := request.ContentLength

	body := &uploadBody{body: request.Body, declared: declared}
	request.Body = body
	// MultipartReader streams: it reads the body as parts are asked for and
	// buffers nothing beyond one small window. ParseMultipartForm and ReadForm
	// would hold the upload in memory or in OS temp files, which the contract
	// forbids, and are not used anywhere in this package.
	multi, err := request.MultipartReader()
	if err != nil {
		writePage(writer, http.StatusBadRequest, formPage(uploadEmptyHeading, uploadEmptyDetail))
		return
	}

	// Refused before a byte is read: the space check needs only the header, and
	// refusing here is what keeps a 10 GB upload to a nearly full disk from
	// being accepted and then failing at 90 percent.
	if err := r.destination.CheckSpace(declared); err != nil {
		// Both an insufficient-space answer and a failure to learn the space
		// refuse: unknown free space is not permission to write.
		r.lane.publishProgress(noticeEvent(r.sessionID, transfer.NoticeReceiveTooLarge))
		writePage(writer, http.StatusRequestEntityTooLarge, formPage(uploadLargeHeading, uploadLargeDetail))
		return
	}

	// The session is claimed by the first file part, not by the first byte. A
	// body with no file part is a mistake the phone can fix by choosing a file,
	// and must not cost it the link.
	first, err := body.nextFilePart(multi)
	if err != nil {
		writePage(writer, http.StatusBadRequest, formPage(uploadEmptyHeading, uploadEmptyDetail))
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

	// From here the connection is a claimed upload. Its read deadline is no
	// longer net/http's whole-request one: it is re-armed after every read that
	// delivers a byte, so only a stall -- never a long transfer -- ends it.
	ctx, stop := context.WithCancel(r.ctx)
	defer stop()
	stopWatch := context.AfterFunc(request.Context(), stop)
	defer stopWatch()

	progress := newMeter(declared, true, r.now, func(snapshot transfer.ProgressSnapshot) {
		r.lane.publishProgress(receiveProgressEvent(r.sessionID, snapshot, r.destination.Snapshot().FilesSaved))
	})
	body.arm(http.NewResponseController(writer), r.timeouts.uploadIdleOrDefault(), progress)

	err = r.savePartsAndDrain(ctx, body, multi, first, progress)
	snapshot := progress.snapshot()
	saved := r.destination.Snapshot().FilesSaved

	if err == nil {
		// The result page reports the count the destination holds -- the same
		// number the desktop is about to be told -- and is rendered before any
		// header is sent.
		writePage(writer, http.StatusOK, resultPage(uploadDoneHeading, savedCount(saved)))
		r.finalizeAfterResponse(request, completeEvent(r.sessionID, snapshot))
		return
	}

	if r.ctx.Err() != nil {
		// The coordinator's own teardown or Cancel. It already owns the outcome,
		// so this reports nothing -- and the response has no honest status to
		// carry, so the connection is broken rather than answered.
		r.finish(nil)
		panic(http.ErrAbortHandler)
	}

	// A genuine failure only this server saw: a dropped connection, an oversize
	// body, an inactivity timeout, or a write error. Completed files stay and the
	// partial one is already gone; the phone is told how many were saved and that
	// the upload did not finish, and the desktop is told the same through the
	// terminal event once the response is out.
	writePage(writer, http.StatusInternalServerError, resultPage(uploadIncompleteHeading, "The upload stopped before it finished. "+savedCount(saved)))
	cause := transfer.WrapError(transfer.ErrTransferFailed, "the upload did not complete", err)
	r.finalizeAfterResponse(request, failedEvent(r.sessionID, snapshot, cause))
}

// savePartsAndDrain writes every file part to the destination, then reads the
// rest of the body so the progress meter ends at exactly the declared length. A
// body that ends short of it is an incomplete upload even when the multipart
// terminator arrived.
func (r *run) savePartsAndDrain(ctx context.Context, body *uploadBody, multi *multipart.Reader, part filePart, progress *meter) error {
	for {
		if _, err := r.destination.SaveFile(ctx, part.name, part.part); err != nil {
			return err
		}
		// Each saved file is an event the desktop wants immediately, not at the
		// next 4 Hz tick.
		r.lane.publishProgress(receiveProgressEvent(r.sessionID, progress.snapshot(), r.destination.Snapshot().FilesSaved))

		next, err := body.nextFilePart(multi)
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		part = next
	}
	if _, err := io.Copy(io.Discard, body); err != nil {
		return err
	}
	if body.read != body.declared {
		return io.ErrUnexpectedEOF
	}
	return nil
}

// uploadIdleOrDefault covers a run built without the production defaults.
func (t serverTimeouts) uploadIdleOrDefault() time.Duration {
	if t.uploadIdle > 0 {
		return t.uploadIdle
	}
	return uploadIdleTimeout
}

// filePart is one file part of the expected form field and the phone's raw
// name for it. The name is untrusted; the destination sanitizes it.
type filePart struct {
	part *multipart.Part
	name string
}

// filePartOf reports whether a part is a file part of the expected field. The
// Content-Disposition is parsed here rather than through Part.FileName, which
// applies the host's filepath.Base: this must decide the same way on Windows and
// macOS, and the destination's own sanitizer removes separators uniformly.
func filePartOf(part *multipart.Part) (filePart, bool) {
	disposition, params, err := mime.ParseMediaType(part.Header.Get("Content-Disposition"))
	if err != nil || disposition != "form-data" || params["name"] != uploadField {
		return filePart{}, false
	}
	name := params["filename"]
	if name == "" {
		// A browser with no file chosen still sends an empty-named file part.
		return filePart{}, false
	}
	return filePart{part: part, name: name}, true
}

// uploadBody is the request body as the multipart reader sees it. It counts
// every byte, refuses any past the declared length, bounds how much a part's
// headers may consume, and -- once the upload is claimed -- re-arms the read
// deadline after every read that delivered a byte.
type uploadBody struct {
	body     io.ReadCloser
	declared int64
	read     int64
	err      error

	inHeader    bool
	headerBytes int64

	controller *http.ResponseController
	idle       time.Duration
	meter      *meter
}

// arm switches the body from the pre-claim phase, bounded by net/http's
// whole-request deadline, to the claimed phase, bounded by inactivity. The bytes
// already read are credited to the meter so the total is exact.
func (b *uploadBody) arm(controller *http.ResponseController, idle time.Duration, progress *meter) {
	b.controller, b.idle, b.meter = controller, idle, progress
	progress.record(int(b.read))
	b.refreshDeadline()
}

func (b *uploadBody) refreshDeadline() {
	if b.controller == nil {
		return
	}
	// An error here means the connection cannot take a deadline (a test double,
	// say). The stall bound is then absent rather than wrong, and the request's
	// own context still ends the upload.
	_ = b.controller.SetReadDeadline(time.Now().Add(b.idle))
}

func (b *uploadBody) Read(p []byte) (int, error) {
	if b.err != nil {
		return 0, b.err
	}
	n, err := b.body.Read(p)
	if n > 0 {
		b.read += int64(n)
		if b.read > b.declared {
			// More than was declared. net/http will not deliver this, so it
			// guards against an alternate transport; either way the excess is
			// never handed on, and nothing past the declared length is saved.
			excess := b.read - b.declared
			if excess > int64(n) {
				excess = int64(n)
			}
			n -= int(excess)
			b.read = b.declared
			b.err = errBodyOverrun
			if n > 0 && b.meter != nil {
				b.meter.record(n)
			}
			return n, b.err
		}
		if b.inHeader {
			b.headerBytes += int64(n)
			if b.headerBytes > maxPartHeaderBytes {
				b.err = errPartHeaders
				return 0, b.err
			}
		}
		if b.meter != nil {
			b.meter.record(n)
			b.refreshDeadline()
		}
	}
	return n, err
}

func (b *uploadBody) Close() error { return b.body.Close() }

// nextFilePart advances to the next file part of the expected field, reading and
// discarding every other part on the way. It returns io.EOF, unwrapped, only for
// a clean end of the multipart stream; a truncated stream is a different error,
// which is what keeps a cut-off upload from reading as a finished one.
func (b *uploadBody) nextFilePart(multi *multipart.Reader) (filePart, error) {
	for {
		b.inHeader, b.headerBytes = true, 0
		part, err := multi.NextPart()
		b.inHeader = false
		if b.err != nil {
			return filePart{}, b.err
		}
		if err != nil {
			return filePart{}, err
		}
		if candidate, ok := filePartOf(part); ok {
			return candidate, nil
		}
		if _, err := io.Copy(io.Discard, part); err != nil {
			return filePart{}, err
		}
	}
}
