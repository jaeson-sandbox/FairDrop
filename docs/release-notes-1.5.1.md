# FairDrop 1.5.1

FairDrop 1.5.1 lets you send up to 10,000 items in one go. Version 1.5.0 allowed 16.

- **Many more items per send.** Drop or add up to 10,000 files and folders, for example a whole batch of photos. The selection list scrolls, and its count shows how many of the 10,000 you have chosen. A selection over 10,000 is refused, never trimmed. Inside the ZIP, item numbers widen with the count (`FairDrop/001-…` for 100 or more items, `FairDrop/0001-…` for 1,000 or more), so names stay distinct and sorted in the order you chose.
- **Checked in full, sent one at a time.** FairDrop still checks the whole selection before it starts sharing: missing items, duplicates, and a folder together with something inside it are refused up front. To support large selections, items are then opened one at a time as the download reaches them, rather than all at once. If an item is changed, replaced or removed after you press **Send**, the download stops at that item and the receiver does not get a completed ZIP. Previously this was caught before the download started.
- **One file or one folder is unchanged.**

FairDrop transfers to one receiver over plain HTTP on a trusted LAN. A capability URL limits blind discovery; it does not protect against an observer on that network. There is no encryption, authentication or relay. Sender-side completion does not prove the browser saved the bytes.

The Windows executable is unsigned. The macOS app is ad-hoc signed only for launch, with no Developer ID signature or notarization. There is no auto-update or Linux packaging. SHA-256 sidecars let you check that downloaded assets match the release assets. A checksum proves integrity, not authenticity: the checksum and binary come from the same release pipeline. It is not a signature.
