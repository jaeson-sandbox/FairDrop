# Epic 6 Context: Replace the Placeholder App Icon

<!-- Compiled from planning artifacts. Edit freely. Regenerate with compile-epic-context if planning docs change. -->

## Goal

Replace the stock Wails scaffold icon throughout FairDrop's release artifacts so the executable, Windows taskbar, NSIS installer, and macOS Dock identify the application with FairDrop's own mark. The result must be reproducible from committed source material, preserve the intended transparent artwork, and verify what actually ships on supported release platforms rather than only checking build inputs.

## Stories

- Story 6.1: Replace the Placeholder App Icon
- Story 6.2: Pin What Ships, Not the Template
- Story 6.3: Verify and Release 1.1.0

## Requirements & Constraints

The committed icon assets must represent the selected FairDrop mark at all operating-system sizes, retain transparency around the mark, and remain visually faithful when downsampled. The master asset must carry an alpha channel and be reproducible from the committed candidate source and derivation script; source provenance must be checked so a missing or substituted candidate cannot silently pass.

The generated ICO must contain the complete required size set and use the repository's established Wails icon-generation path. Verification must reject stale, incomplete, opaque, incorrectly transformed, or otherwise substituted assets. Checks should measure the intended properties and use mutation coverage so a passing test demonstrates the release guarantee rather than merely comparing a file to itself.

Release verification must inspect the built Windows executable's embedded resources. It must confirm that the executable carries the expected icon payloads and release identity, and that missing local build output is an explicit skip rather than a false pass. The CI drift check must also keep the committed build assets present. The complete native verification gate remains required across Windows and macOS; manual visual checks are optional under the personal release policy and must not be represented as automated evidence. Story 6.3 closes the remaining proof and documentation gaps and requires native CI, merge, tag, artifact transit, checksum, and publication evidence before 1.1.0 becomes a released version.

## Technical Decisions

Keep the artwork as build input under `build/`, with the original candidate and deterministic derivation script committed for future higher-fidelity regeneration. Do not assert byte identity that depends on a particular Pillow version; assert decoded dimensions, alpha and color behavior, geometry, provenance, and calibrated per-size similarity instead.

Use the existing Wails v2 packaging flow and its literal icon-generation inputs. Do not add a new image or resource dependency. For Windows product verification, use the Go standard library's `debug/pe` section information plus a manual three-level PE resource-directory walk to read `RT_ICON`, `RT_GROUP_ICON`, and `RT_VERSION`. Compare decoded icon resources to the committed ICO at matching sizes and read version strings from the resource block, because the language-neutral table is not reliably covered by higher-level file-version APIs.

Preserve the architecture's native release policy: build releases on native Windows and macOS runners, keep stable Wails v2 and locked dependencies, and treat release artifacts as build outputs rather than runtime product state. Linux packaging, signing, notarization, and auto-update remain outside this epic. Existing release identity values in `wails.json` are the source of truth for the executable's embedded identity.

## UX & Interaction Patterns

The icon is part of FairDrop's recognizable desktop identity. It should support the Paper Relay direction: a compact handoff tool with warm paper surfaces, restrained terracotta action color, quiet editorial presentation, and standard OS window chrome. Desktop launch and system surfaces should show the same FairDrop mark without implying storage, receiver identity, encryption, or a separate product flow.

## Cross-Story Dependencies

Story 6.2 depends on Story 6.1's committed master, ICO, calibrated similarity tolerances, and resource-generation path. The executable checks also depend on the existing native Wails build and CI gate; they should skip cleanly when a local executable is absent while remaining exercised by the Windows build job. The release identity check shares `wails.json` with the existing application packaging configuration.
