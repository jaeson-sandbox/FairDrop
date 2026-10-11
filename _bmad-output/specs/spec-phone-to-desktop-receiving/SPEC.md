---
id: SPEC-phone-to-desktop-receiving
companions:
  - receive-contract.md
  - ../spec-fairdrop/SPEC.md
  - ../spec-receiver-handoff/receiver-protocol.md
sources: []
---

> **Canonical contract.** This SPEC and the files in `companions:` are the complete, preservation-validated contract for what to build, test, and validate. Source documents listed in frontmatter are for traceability — consult them only if you need narrative rationale or prose color this contract intentionally omits.

# Phone-to-desktop receiving

## Why

FairDrop only sends from the desktop. Moving photos or files from a phone to the same computer still needs cables, cloud services or a messaging app. A desktop user should be able to show a QR code, let a nearby phone browser pick files, and get those files saved into a folder they chose. This is roadmap slice 4. It is the first FairDrop feature that writes to the user's disk, so where files go, how big an upload may be, how files are named and what a failure leaves behind are all part of the contract.

## Capabilities

- **CAP-1**
  - **intent:** A desktop user can open a receive session into a folder they choose and show a one-time QR code and link.
  - **success:** Choosing **Receive** opens a folder chooser first. No listener or QR code exists until a folder is chosen, and dismissing the chooser changes nothing. A session starts only from IDLE through the existing coordinator lifecycle. Cancelling before an upload writes nothing.
- **CAP-2**
  - **intent:** One phone browser can choose files on a script-free page and upload them once to the desktop.
  - **success:** The first valid upload claims the session. Files stream to disk with bounded memory into a freshly created, time-named subfolder. Unsafe or duplicate names are renamed, not refused. The phone's result page and the desktop both state exactly how many files were saved.
- **CAP-3**
  - **intent:** The desktop user can follow a receive and finish it with an honest result.
  - **success:** Progress is shown against the declared upload size. Cancel or failure keeps completed files and removes only the file being written, and the outcome states the saved count and whether the upload was complete. **Show in Folder** opens the subfolder. Received files carry the OS download marking and are never executable.

## Constraints

- One process, one live session and one counterpart. Sending and receiving never run at the same time, and each receive session accepts one upload. Receive reuses the capability token, QR code, beacon, server lifecycle, bounded teardown and the generic 404/423 responses.
- Possessing the one-time URL after the user chose a destination is the approval. There is no per-upload Accept step, and the phone page uses no scripts or external resources.
- The upload's declared size must fit on the destination volume with at least 3 GiB still free afterwards. Otherwise it is refused before anything is written. There is no file-count cap. A missing declared size is refused, and bytes beyond the declared size abort the upload.
- Uploads stream from the request straight to their destination files through bounded buffers. Nothing is buffered in memory or written to OS temp storage. The current 20-second whole-request read timeout would cut off uploads, so upload bodies use a progress-based inactivity bound instead. Download behaviour is unchanged.
- Writes stay inside one newly created subfolder of the chosen destination. The subfolder and every file are created exclusively, without following links, and an existing file is never opened for writing, replaced or appended to. A file counts as saved only after it was written to a temporary name, flushed, closed, and renamed to its final name without replacing anything.
- Persistence wording must be honest. FairDrop still stores no settings, history, logs or staged copies, but a receive session saves the received files into the chosen folder. The destination path never reaches HTTP responses, mDNS, diagnostics or the phone.
- The request, naming, storage and outcome rules are in receive-contract.md. Existing send-side guarantees are unchanged.

## Non-goals

- A desktop Accept/Reject step, uploading folders from the phone, resuming an upload, several uploads or uploaders per session, and remembering a destination between sessions.
- Text or link sharing, connection recovery, TLS or authentication, malware scanning, and converting file formats (e.g. HEIC).

## Success signal

On native Windows and macOS, the user chooses **Receive** and picks a folder, and a phone scans the QR code. The phone uploads three photos, two of them with the same name. The desktop shows progress, then reports 3 files saved in a new `FairDrop YYYY-MM-DD HH.MM` subfolder, with the duplicate saved as `name (1).ext`. **Show in Folder** opens that subfolder, and the files carry the OS download marking. If Wi-Fi drops partway through, only completed files remain, the partial file is gone, and the desktop reports the upload as incomplete with the exact saved count.

## Assumptions

- A refusal for space does not use up the session. The phone sees a fixed "too large for this computer" page, and the desktop keeps waiting and notes that an upload was refused for space.
- OS download marking is best-effort. On filesystems that cannot store it (FAT, exFAT), the file is kept and the outcome warns that the marking was not applied.
- The subfolder is created at the first file part, so an abandoned session leaves nothing on disk.
- The canonical FairDrop SPEC is re-derived when this slice is implemented. That changes the persist-nothing constraint and the receiving non-goal, as was done for collections.
