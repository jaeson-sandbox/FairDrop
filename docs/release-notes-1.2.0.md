# FairDrop 1.2.0

FairDrop 1.2.0 rebuilds the interface on the Quartz spine, replacing Terracotta Linen. Every
view, control, and surface in the window is restyled; a person updating will see a different
product before they notice anything else.

Underneath the new interface, this release also fixes functional problems a user could feel:

- Tab now reaches the browse control on macOS. It previously could not be reached by keyboard
  at all — `WKPreferences.tabFocusesLinks` defaults off, so the control was invisible to Tab
  regardless of what the rest of the keyboard path did.
- The completion screen now shows what was sent and how much, instead of only reporting that
  the transfer finished.
- The direct-link field no longer clips its own URL.
- The window resizes without the link field lagging behind it.

Transfer behavior is unchanged from 1.1.0: one file or folder, one receiver, plain HTTP on a
trusted LAN, and no persisted history.

The Windows executable is unsigned. The macOS app is ad-hoc signed for launch but is not
Developer ID signed or notarized. There is no installer, auto-update, Linux package, hostile-network
protection, or end-to-end encryption.
