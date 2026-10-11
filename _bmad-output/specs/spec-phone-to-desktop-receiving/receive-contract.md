# Receive contract

## Desktop interaction

Idle gains a **Receive** action alongside the existing send entry points. Activating it opens the native folder chooser at the user's Downloads folder. Dismissing the chooser is a no-op. A chosen folder is held in memory for this session only, then admitted through the coordinator like Stage. Admission is allowed only from IDLE, uses the same single-flight and busy rules, and is refused if the folder is missing, not a directory, or link-like.

The waiting view shows the QR code and the link (revealed on demand, like Staged), names the destination folder by its basename only, and offers Cancel. While an upload is running, the view shows bytes received against the declared size and a count of files saved so far. The outcome view states one of: **N files saved**, **Upload incomplete — N files saved**, **Upload cancelled — N files saved**, or a failure with no files saved. It offers **Show in Folder** whenever the subfolder exists, and a way back to Idle. A marking warning appears on the outcome when it applies. Use Quartz tokens, the existing focus and announcement routing, and no button-label ellipses. Folder and outcome choices are never persisted.

**Show in Folder** opens the session subfolder in Finder or File Explorer through a bound Go command. The path goes to the OS as an argument, never through a shell, and the command refuses when the session has no subfolder.

## HTTP

The receive capability path uses its own route prefix with the existing 128-bit token and constant-time match. Wrong token, wrong route, malformed paths, HEAD and other methods get the existing generic 404. Once claimed, a valid request gets 423 while the listener is live.

| Request/state | Response | Side effect |
| --- | --- | --- |
| Exact valid GET, unclaimed | 200 script-free upload page | None |
| Exact valid POST, `multipart/form-data`, declared size passes the space check | Stream, then a result page | Claims the session and writes files |
| POST without a declared size | 411 fixed page | None; session stays waiting |
| POST whose declared size fails the 3 GiB reserve check | 413 fixed "too large for this computer" page | None; session stays waiting; desktop notes the refusal |
| POST that is not multipart, or has no file parts | 400 fixed page | None; session stays waiting |
| Already claimed | 423 while the listener lives | None |

The upload page shows FairDrop, a native `<input type="file" multiple>` and an **Upload** button, posting to a same-origin relative action. It keeps the receiver page's response headers: no-store, no-referrer, nosniff, and a CSP with no scripts, framing or external resources that allows `form-action 'self'`. Styling is responsive at 320 CSS pixels, with visible focus, light/dark and forced-colors support. The result page is rendered after the outcome is known. It reports the saved count and, when incomplete, says so. It never shows paths, and it escapes any echoed names.

Only file parts in the expected form field are written. Other parts are read and discarded within the declared size. Part headers are bounded. Upload bodies use a progress-based inactivity bound rather than the whole-request read timeout; the header timeout and header-size limit stay as they are. Desktop Cancel and teardown close the connection within the existing bounds.

## Naming

Each upload writes into a new subfolder of the chosen destination named `FairDrop YYYY-MM-DD HH.MM` in local time, with ` (n)` appended if that name exists. The subfolder is created with exclusive creation when the first file part arrives.

Phone-supplied names are untrusted. Keep only the final component after both `/` and `\` separators. Remove control, format and bidirectional characters. Replace characters that Windows or macOS cannot store, avoid Windows device names, and trim trailing dots and spaces. Truncate on rune boundaries to fit 255 UTF-8 bytes, keeping the extension where possible. Fall back to `file` when nothing usable remains. Within one upload, names that collide (case-insensitively) get ` (1)`, ` (2)` before the extension. A name is never refused.

## Storage

Each file part is written through bounded buffers to an exclusively created temporary name inside the subfolder. It is flushed and closed, then renamed to its final name without replacing anything; if the final name was taken in the meantime, the next ` (n)` is used. Files and the subfolder are created without following links and with non-executable permissions (files 0644, directories 0755 before umask). Received files get the OS download marking: `com.apple.quarantine` on macOS, and a `Zone.Identifier` stream with ZoneId=3 on Windows. Marking is best-effort, and a failure produces an outcome warning, not data loss.

Free space is checked against the declared size plus a 3 GiB reserve before the first byte is written. Disk-full or write errors during the upload fail it under the partial-failure rule below.

## Failure, cancel and honesty

A file counts as saved only after its rename succeeds. On a dropped connection, phone abort, desktop Cancel, oversize body, inactivity timeout or write error: remove the temporary file being written, keep every saved file, stop reading, and report the exact saved count. If nothing was saved, remove the empty subfolder. Never report an incomplete upload as complete. Teardown follows the existing bounds, and an elapsed bound is reported, never treated as quiet success.

Wording on the desktop, the phone page and in the documentation must say that files were **saved to this computer** only after rename. It must not claim persistence beyond the filesystem's own durability, and it must not claim that nothing is written to disk in receive mode.

## Required proof

- Real HTTP tests: GET leaves the session waiting; one multipart POST saves exact bytes; a second POST gets 423; 411/413/400 leave the session waiting and write nothing.
- Space-reserve boundary, oversize body abort, inactivity abort, and desktop Cancel mid-file. Each keeps the completed files and removes the partial one.
- Name sanitization and in-upload dedupe, subfolder collision, and proof that an existing file is never opened for writing.
- Bounded memory over a large upload, and no use of OS temp storage.
- Quarantine/MOTW present on native macOS/Windows CI; the warning path where marking is unsupported.
- Show in Folder argument passing (no shell).
- Mutation proof that names each broken guarantee, with complete logs, in sibling evidence.
