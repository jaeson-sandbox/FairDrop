package server

import (
	"bytes"
	"html/template"
	"net/http"
	"strconv"

	"fairdrop/internal/transfer"
)

// The only dynamic fields are Stage-time display metadata. html/template
// escapes them for their HTML contexts; the action is the current path and
// never comes from Host, a query, or the staged source path.
var landingTemplate = template.Must(template.New("receiver").Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>Download with FairDrop</title><style>
:root{color-scheme:light dark;font-family:system-ui,-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;background:#F2F2F4;color:#1A1A1C}
*{box-sizing:border-box}body{margin:0;min-height:100vh;display:grid;grid-template-columns:minmax(0,1fr);place-items:center;padding:24px 16px}
main{min-width:0;width:min(100%,460px);overflow-wrap:anywhere;padding:clamp(24px,6vw,40px);border:1px solid #D6D6D6;border-radius:20px;background:#FFFFFF;box-shadow:0 12px 32px #1A1A1C12}
.brand{font-size:14px;font-weight:700;letter-spacing:.08em;text-transform:uppercase;color:#9C5636}
h1{font-size:clamp(26px,7vw,34px);line-height:1.15;margin:28px 0 8px}
.name{font-size:20px;font-weight:650;line-height:1.35;overflow-wrap:anywhere;word-break:break-word;unicode-bidi:isolate;margin:0}
.detail{color:#65656B;line-height:1.5;margin:12px 0 28px}button{font:inherit;font-weight:700;min-height:48px;width:100%;border:0;border-radius:12px;background:#9C5636;color:#FFFFFF;cursor:pointer}
button:hover{background:#7F4428}button:focus-visible{outline:2px solid #9C5636;outline-offset:4px;box-shadow:0 0 0 2px #FFFFFF}
.note{font-size:14px;line-height:1.55;color:#65656B;margin:24px 0 0}
@media(prefers-color-scheme:dark){:root{background:#161618;color:#F2F2F4}main{background:#1F1F22;border-color:#47474A;box-shadow:none}.brand{color:#E39B70}.detail,.note{color:#9C9CA4}button{background:#E39B70;color:#2B1206}button:hover{background:#F0B694}button:focus-visible{outline-color:#E39B70;box-shadow:0 0 0 2px #1F1F22}}
@media(forced-colors:active){main{border:1px solid CanvasText}button{border:1px solid ButtonText}button:focus-visible{outline:3px solid Highlight}}
</style></head><body><main><div class="brand">FairDrop</div><h1>{{.Heading}}</h1>
<p class="name" dir="auto"><bdi>{{.Name}}</bdi></p>
<p class="detail">{{.Detail}}</p><form method="post" action=""><button type="submit">{{.Button}}</button></form>
<p class="note">{{.Trust}}</p>
</main></body></html>`))

// Receiver-owned copy registry, tabulated under Receiver page copy in the
// Quartz EXPERIENCE. The desktop TypeScript registry has no receiver reader.
const (
	receiverHeading          = "Ready to download"
	receiverButton           = "Download"
	receiverFilePrefix       = "File · "
	receiverFolderDetail     = "Folder · Downloads as a ZIP."
	receiverCollectionSuffix = " · Downloads as a ZIP."
	receiverSizeUnavailable  = "Size unavailable"
	receiverTrust            = "Use only on a local network you trust. FairDrop keeps no copy; the receiving device keeps what it downloads. The first device to download gets this item."
)

type landingData struct {
	Heading string
	Name    string
	Detail  string
	Button  string
	Trust   string
}

func (r *run) landing(writer http.ResponseWriter, request *http.Request) {
	if request.Context().Err() != nil || r.ctx.Err() != nil {
		writeStatus(writer, http.StatusNotFound)
		return
	}
	detail := receiverFilePrefix + formatLogicalSize(r.item.LogicalSize)
	if r.item.Kind == transfer.ItemDirectory {
		detail = receiverFolderDetail
	}
	if r.item.Kind == transfer.ItemCollection {
		detail = formatLogicalSize(r.item.LogicalSize) + receiverCollectionSuffix
	}
	var body bytes.Buffer
	if err := landingTemplate.Execute(&body, landingData{receiverHeading, r.item.Name, detail, receiverButton, receiverTrust}); err != nil {
		writeStatus(writer, http.StatusInternalServerError)
		return
	}
	if request.Context().Err() != nil || r.ctx.Err() != nil {
		writeStatus(writer, http.StatusNotFound)
		return
	}
	header := writer.Header()
	header.Set("Content-Type", "text/html; charset=utf-8")
	header.Set("Cache-Control", "no-store")
	header.Set("Referrer-Policy", "no-referrer")
	header.Set("X-Content-Type-Options", "nosniff")
	header.Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; form-action 'self'; base-uri 'none'; frame-ancestors 'none'")
	header.Set("Content-Length", strconv.Itoa(body.Len()))
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(body.Bytes())
}

func formatLogicalSize(size int64) string {
	if size < 0 {
		return receiverSizeUnavailable
	}
	if size == 1 {
		return "1 byte"
	}
	if size < 1000 {
		return strconv.FormatInt(size, 10) + " bytes"
	}
	units := []string{"KB", "MB", "GB", "TB"}
	value := float64(size)
	unit := -1
	for value >= 1000 && unit < len(units)-1 {
		value /= 1000
		unit++
	}
	return strconv.FormatFloat(value, 'f', 1, 64) + " " + units[unit]
}
