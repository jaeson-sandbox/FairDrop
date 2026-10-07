# FairDrop 1.4.0

FairDrop 1.4.0 gives the receiver an explicit Download step and lets the sender offer a completed item again.

- **See the item before downloading.** Opening a valid link with GET now shows a landing page with the offered file or folder information. Previewing reserves nothing and does not read the payload. Press **Download** to submit a POST; the first valid POST claims the one-shot transfer. A claim can still fail before or during streaming, and another claim cannot receive a second copy. Sender-side completion does not prove the browser saved the bytes.
- **Send Again after a completed transfer.** FairDrop retains only the completed item's path in memory while the result remains available. **Send Again** revalidates the current item through the normal selection process and creates a fresh link and QR code. It does not replay a snapshot of the prior transfer. If revalidation fails, choose the item again. The old link cannot be reused; there is no persistent history, staged copy or resume.

**Direct HTTP clients must change from GET to POST** at the capability URL to download. GET now previews metadata, so a client that relied on GET starting the download will receive HTML instead. This is an intentional compatibility change.

For a direct client, replace the placeholder with the link shown by FairDrop and choose a local output filename:

```sh
curl --fail --request POST --output chosen-filename 'CAPABILITY_URL'
```

FairDrop transfers one file or folder to one receiver over plain HTTP on a trusted LAN. A capability URL limits blind discovery; it does not protect against an observer on that network. There is no encryption, authentication or relay.

The Windows executable is unsigned. The macOS app is ad-hoc signed only for launch, with no Developer ID signature or notarization. There is no auto-update or Linux packaging. SHA-256 sidecars let you check that downloaded assets match the release assets. A checksum proves integrity, not authenticity: the checksum and binary come from the same release pipeline. It is not a signature.
