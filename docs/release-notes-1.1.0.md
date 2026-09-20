# FairDrop 1.1.0

FairDrop 1.1.0 replaces the stock Wails placeholder with the FairDrop mark in the
Windows executable and macOS app. The Windows gate now opens the built executable
and verifies its icon payloads, product identity, FileVersion, and readable US-English
version table. This release also makes the icon derivation reproducible from the
committed source render and adds mutation proofs for the asset and executable guards.

Transfer behavior is unchanged from 1.0.0: one file or folder, one receiver, plain
HTTP on a trusted LAN, and no persisted history.

The Windows executable is unsigned. The macOS app is ad-hoc signed for launch but is
not Developer ID signed or notarized. There is no installer, auto-update, Linux
package, hostile-network protection, or end-to-end encryption.
