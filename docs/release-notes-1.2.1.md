# FairDrop 1.2.1

FairDrop 1.2.1 fixes a crash. On macOS, dragging the QR code on the "Ready to pass
along" screen terminated the application, taking the staged transfer with it. The QR
is now not draggable, which it never had reason to be.

The crash is older than 1.2.0. The QR has been a plain image since the first release,
so 1.0.0 and 1.1.0 are affected the same way. If you are on either of those, this is
the release to take.

The underlying fault is in the Wails runtime FairDrop is built on: its native
drag-and-drop handler reads every URL off the drop pasteboard and treats each one as a
file path. Dragging the QR puts that image's `data:` URL on the pasteboard, which is
not a file path, and the process ends. FairDrop works around it by never starting the
drag; the runtime behaviour is unchanged and is recorded in the project's notes so a
future runtime upgrade can be checked against it.

Nothing else changed. The Quartz interface, the transfer behavior, and every limitation
listed for 1.2.0 are the same: one file or folder, one receiver, plain HTTP on a trusted
LAN, and no persisted history.

The Windows executable is unsigned. The macOS app is ad-hoc signed for launch but is not
Developer ID signed or notarized. There is no installer, auto-update, Linux package,
hostile-network protection, or end-to-end encryption.
