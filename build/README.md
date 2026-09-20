# Build Directory

The build directory is used to house all the build files and assets for your application. 

The structure is:

* bin - Output directory
* darwin - macOS specific files
* windows - Windows specific files

## App Icon

- `appicon.png` - the single art source for every platform's icon. `wails build` derives macOS's
  `iconfile.icns` from it on every build, and Windows' `icon.ico` from it the one time `icon.ico` is
  absent (see the Windows section below).
- `appicon-source.jpg` - the original candidate JPEG `appicon.png` was derived from, committed
  alongside it so the master is reproducible rather than a one-off nobody can regenerate.
- `../scripts/build-appicon.py` - the derivation itself: crops the plaque out of `appicon-source.jpg`
  and synthesises real alpha, since none of the owner's candidate renders carries any. It is not part
  of any automated pipeline -- its output is committed, and CI checks the committed files, not the
  script.

  **Requires Pillow** (`pip install Pillow`); the committed master was produced with Pillow 12.3.0.
  The script deliberately does not guarantee byte-identical output across Pillow versions, so the
  version is recorded here rather than pinned in a lockfile the repo does not otherwise have.

  To re-derive after replacing `appicon-source.jpg`, **in this order** — the
  script refuses any render it has not been told about, so the constants come first:

  ```sh
  # 1. Point the pins at the new render. Both must change together:
  #    EXPECTED_SOURCE_SHA256 in scripts/build-appicon.py
  #    wantSourceSHA256 (and wantSourceWidth/Height if the size differs) in appicon_test.go
  # 2. If the render is not 2816x1536, re-fit CROP_BOX and CORNER_RADIUS too.
  python scripts/build-appicon.py     # runs from any directory; rewrites build/appicon.png
  rm build/windows/icon.ico           # Wails only generates it when it is ABSENT
  wails build                         # regenerates icon.ico from the new master
  # 3. Re-measure measuredEntryDistance255 in appicon_test.go: new artwork moves
  #    all six same-artwork distances, and until they are updated the first
  #    `go test` reports a correct tree as a stale-icon bug.
  # 4. COMMIT all three binaries, THEN re-derive the evidence. The driver
  #    restores assets from HEAD between cases, so running it against
  #    uncommitted work would discard exactly what you just produced.
  python scripts/verify-asset-mutations.py   # after committing, never before
  ```

  **Commit all three binaries** — `appicon-source.jpg`, `appicon.png` and `windows/icon.ico`.
  Leaving a stale `icon.ico` beside a new master is the exact failure this whole section exists to
  prevent, and it is what shipped the Wails placeholder through v1.0.0.
  `TestAppIconMasterMatchesIcoFreshness` compares every `.ico` entry against the master and fails
  if any disagrees; a uniform 1/255 tonal change is enough to trip it.

## Mac

The `darwin` directory holds files specific to Mac builds.
These may be customised and used as part of the build. To return these files to the default state, simply delete them
and
build with `wails build`.

The directory contains the following files:

- `Info.plist` - the main plist file used for Mac builds. It is used when building using `wails build`.
- `Info.dev.plist` - same as the main plist file but used when building using `wails dev`.

## Windows

The `windows` directory contains the manifest and rc files used when building with `wails build`.
These may be customised for your application. To return these files to the default state, simply delete them and
build with `wails build`.

- `icon.ico` - The icon used for the application. `wails build` generates it from `appicon.png`, but **only
  when `icon.ico` is absent** -- that is the sole trigger, not a fallback alongside some other path. Once the
  file exists, `wails build` never regenerates or overwrites it, even after `appicon.png` changes. To pick up
  a new `appicon.png`, delete `icon.ico` first and rebuild. If you wish to use a different icon directly,
  replace this file with your own instead.
- `installer/*` - The files used to create the Windows installer. These are used when building using `wails build`.
- `info.json` - Application details used for Windows builds. The data here will be used by the Windows installer,
  as well as the application itself (right click the exe -> properties -> details)
- `wails.exe.manifest` - The main application manifest file.