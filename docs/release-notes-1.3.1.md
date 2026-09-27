# FairDrop 1.3.1

FairDrop 1.3.1 polishes two things in the 1.3.0 redesign.

- **Cancelling now shows a notification instead of a banner.** When you cancel, a small
  translucent card slides down at the top of the window — "Transfer canceled", with "Ready for
  another file or folder." beneath it — and slides away on its own after a few seconds. It
  pauses while the pointer is over it, and with Reduce Motion on it fades instead of sliding.
  It no longer pushes the rest of the window down, and it leaves nothing behind.
- **The direct link field looks right when you click it.** Its focus ring was being cut off,
  leaving a band above and below the field; it now draws as a complete rounded ring. The
  selected link is highlighted in the app's own accent colour, visible in light and dark.

Transfer behavior is unchanged from 1.3.0: one file or folder, one receiver, plain HTTP on a
trusted LAN, and no persisted history.

The Windows executable is unsigned. The macOS app is ad-hoc signed for launch but is not
Developer ID signed or notarized. There is no installer, auto-update, Linux package,
hostile-network protection, or end-to-end encryption.
