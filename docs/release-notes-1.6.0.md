# FairDrop 1.6.0

FairDrop 1.6.0 adds receiving: a phone on the same Wi-Fi can send files to your computer.

- **Receive files from a phone.** Choose **Receive** and pick a folder. FairDrop then shows a one-time QR code. Scan it with the phone, choose the files, and tap **Upload**. Nothing is listening until you have picked a folder, and the phone needs no app: the upload page is a plain web form.
- **Saved safely, never overwritten.** Each upload goes into a new subfolder of the folder you chose, named `FairDrop YYYY-MM-DD HH.MM`. Existing files are never replaced or changed. If two uploaded files have the same name, the second is saved as `name (1).ext`. Unsafe names are cleaned up rather than refused. **Show in Folder** opens the new subfolder.
- **Honest results.** The computer and the phone both report exactly how many files were saved. If the upload stops partway, because Wi-Fi drops, the phone gives up or you press **Cancel**, the files that finished are kept, the one in progress is removed, and the result says the upload was incomplete. A file counts as saved only once it is fully written and in place.
- **Space and safety.** An upload is refused before anything is written unless at least 3 GB would still be free on that disk afterwards. Received files get the usual "downloaded from the internet" marking where the disk supports it, and are never made executable. The link works for one upload.
- **Also in this release:** more space between **Choose File or Folder** and **Send multiple items** on the start screen.

Limits: the destination must be a plain folder you can write to. Folders that are links, or that sit inside OneDrive-style cloud folders, are refused. On macOS, FAT and exFAT drives (many USB sticks) cannot be received into: FairDrop reports the upload as incomplete rather than risk overwriting a file.

FairDrop stores nothing of its own: no settings, history, logs or remembered folder. The only thing it writes is files you choose to receive. Sending and receiving use plain HTTP on a trusted LAN. A capability link limits blind discovery, but it does not protect against an observer on that network. There is no encryption, authentication or relay.

The Windows executable is unsigned. The macOS app is ad-hoc signed only for launch, with no Developer ID signature or notarization. There is no auto-update or Linux packaging. SHA-256 sidecars let you check that downloaded assets match the release assets. A checksum proves integrity, not authenticity: the checksum and binary come from the same release pipeline. It is not a signature.
