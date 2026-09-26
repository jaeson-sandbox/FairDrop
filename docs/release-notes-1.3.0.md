# FairDrop 1.3.0

FairDrop 1.3.0 redesigns every screen. The Quartz look from 1.2.0 stays; what changes is how
much each screen asks you to read, and how it moves from one state to the next.

- **Motion.** Each screen fades and rises into place, menus scale in, and help sections slide
  open and closed. With Reduce Motion on, the same screens appear without moving. On older
  macOS versions whose WebKit cannot animate them, screens simply appear.
- **Idle.** The **Choose File or Folder** button now sits inside the drop zone, and the two help
  sections are one list.
- **Ready to send.** The QR code is the centre of the screen. The link is no longer printed
  underneath it: **Copy Link** copies it without showing it, and **Show Link** reveals it. The
  paragraphs of guidance become two short notes, with the rest under **Trouble connecting?**.
- **Sending.** A progress ring takes the QR code's place, with the amount sent and the speed
  beside it.
- **Sent, and errors.** A finished or failed transfer is now one centred card instead of a
  panel stacked above the drop zone. **Send Another** starts the next one. When a transfer
  fails for a reason that sending again can fix, **Try Again** re-prepares the same file or
  folder — no chooser — and gives you a fresh code. When it cannot, the card offers
  **Choose Another** instead.

Two fixes a person could feel:

- On macOS, clicking **Copy Link** with the mouse now shows **Copied**. The copy always worked,
  but the confirmation never appeared, because WebKit does not focus a button you click. This
  was present in every earlier release.
- No button label ends in an ellipsis any more.

Transfer behavior is unchanged from 1.2.1: one file or folder, one receiver, plain HTTP on a
trusted LAN, and no persisted history. The remembered item behind **Try Again** is held in
memory only and is forgotten when you dismiss the card or send something else.

The Windows executable is unsigned. The macOS app is ad-hoc signed for launch but is not
Developer ID signed or notarized. There is no installer, auto-update, Linux package,
hostile-network protection, or end-to-end encryption.
