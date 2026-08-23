# Trust & Cross-Device Handoff Review

Date: 2026-08-23  
Scope: `DESIGN.md` and `EXPERIENCE.md`, checked against the UX memlog, source extract, Paper Relay / Terracotta Linen working boards, primary-flow wireframe, canonical `SPEC.md`, `ARCHITECTURE-SPINE.md`, `docs/fairdrop-contracts.md`, and `docs/fairdrop-architecture.md`.

## Verdict

**Revise before the UX spines are finalized.** The pair is unusually disciplined about V1 scope: it correctly defines a Windows/macOS native sender, a same-LAN browser receiver, folder-as-ZIP behavior, no receiver app, no cloud/history/settings, plain HTTP, a one-receiver lifecycle, and mobile sending as roadmap-only. It also rejects locks, shields, “secure,” and “encrypted.”

The remaining trust gap is not broad scope drift. It is the consumer meaning of the handoff. The draft does not yet say plainly enough that the URL is a bearer capability claimed by the first device that opens it, that copied links can escape FairDrop, that plain HTTP has a real observer risk, or that sender-side Done cannot prove where the receiver saved the file. Firewall timing/recovery and receiver-browser support are still product decisions. Those gaps make an AirDrop-like promise feel more complete than the actual V1 journey.

| Severity | Count |
|---|---:|
| Critical | 0 |
| High | 5 |
| Medium | 6 |
| Low | 1 |
| **Total** | **12** |

## Findings

### TCH-01 — Plain-HTTP copy names the mechanism but not its consequence

- **Severity:** High
- **Classification:** Documentation/copy defect
- **Spine locations:** `EXPERIENCE.md` § **Voice and Tone**, row “Same trusted network. FairDrop uses plain HTTP”; § **Component Patterns**, **Trusted-LAN Note**; § **Trusted-LAN & Privacy**, first bullet. `DESIGN.md` § **Do's and Don'ts**, row “State trusted-LAN/plain-HTTP limits neutrally.”
- **Evidence:** The internal privacy section correctly says that the capability URL does not protect against a LAN observer, matching canonical `SPEC.md` § **Constraints**, `ARCHITECTURE-SPINE.md` **AD-9**, and `docs/fairdrop-architecture.md` § **HTTP protocol and security envelope**. The example consumer copy only says “plain HTTP.” A normal consumer cannot be expected to infer that traffic is not confidential or that possession of the link is the authorization.
- **Consequence:** “Trusted LAN” can read like a security certification, while “plain HTTP” becomes unexplained jargon. Users may use FairDrop on guest, workplace, hotel, or other networks they do not actually trust.
- **Fix:** Require a plain-language disclosure, not just terminology. Suggested meaning: “Use FairDrop only on a network you trust. The transfer is not encrypted, so someone monitoring this network may be able to observe it.” Keep the existing prohibition on lock/shield/green-safe treatment.

### TCH-02 — “Single-use” does not explain first-requester ownership or link-preview risk

- **Severity:** High
- **Classification:** Documentation defect; link-preview tolerance is a product/protocol decision
- **Spine locations:** `EXPERIENCE.md` § **Component Patterns**, **Direct URL Row**; § **State Patterns**, **Receiver/valid first claim**, **Receiver/competing valid claim**; § **Key Flows**, Flow 1 step 6 and Flow 2 step 4.
- **Evidence:** Canonical `docs/fairdrop-contracts.md` § **Claim and HTTP ordering** says the first exact-token `GET` atomically reserves the transfer and another exact-token request gets `423` only while the listener remains live. The spine exposes only “single-use URL” to the consumer. It never says that the first device or software agent to open the URL wins, that there is no receiver identity/approval step, or that a chat/link-preview service issuing `GET` could claim it.
- **Consequence:** A sender may paste the link into a messaging product that previews it, open it on the wrong device, or share it with several people and assume the intended receiver is selected. FairDrop has no pairing or authenticated receiver identity to correct that assumption.
- **Fix:** Mandate staged copy with the meaning “One device only—the first device to open this link starts the download.” Tell users to open/paste the URL directly in the receiving browser. Separately decide whether V1 accepts link-preview claims as an inherent limitation, adds protocol hardening, or removes general “share this link” implications from product copy.

### TCH-03 — Firewall guidance remains unresolved at the most failure-prone handoff point

- **Severity:** High
- **Classification:** Product decision (already recorded in `.working/finalize-gaps.md`)
- **Spine locations:** `EXPERIENCE.md` § **Information Architecture**, **OS firewall prompt**; § **State Patterns**, **Firewall/allowed** and **Firewall/denied or no eligible LAN**; § **Trusted-LAN & Privacy**, final bullet; § **Key Flows**, Flow 4 steps 1–2 and failure paragraph.
- **Evidence:** The spine requires guidance but explicitly leaves placement and wording open. “First inbound-LAN use” and Flow 4’s launch-time guidance also leave the relationship between in-app explanation and the OS-owned prompt underspecified. The canonical requirement is stronger: the sender must permit the firewall access required for inbound HTTP, and native smoke evidence must cover first launch and firewall guidance.
- **Consequence:** A user can deny an unfamiliar OS prompt, receive only `network_unavailable` or a setup error, and have no trustworthy recovery path. On Windows, vague “allow LAN access” copy may also encourage allowing public networks.
- **Fix:** Approve a cross-platform contract before implementation: when guidance appears, exact reason, Windows private/public-network advice, macOS incoming-connection language, what the UI shows after denial, and where recovery instructions live. A practical V1 default is brief guidance during the first Stage pending state plus actionable post-denial help; release notes alone are insufficient.

### TCH-04 — Done overclaims receiver storage and iPhone Files state

- **Severity:** High
- **Classification:** Documentation/copy defect
- **Spine locations:** `EXPERIENCE.md` § **Component Patterns**, **Done Panel**; § **State Patterns**, **Done**; § **Key Flows**, Flow 1 steps 9–10. The working flow reinforces the issue with “The folder ZIP is ready in iPhone Files.”
- **Evidence:** Backend `transfer-complete` proves the response stream completed according to the sender/server contract. It does not prove where the browser saved the attachment, that iOS Files has committed it, or that the receiver opened the ZIP. Browser/OS save behavior is explicitly outside the FairDrop receiver surface.
- **Consequence:** The sender sees a stronger success claim than the product can know. If Safari prompts, renames, discards, or cannot surface the file as expected, FairDrop still says it is ready in Files.
- **Fix:** Define Done as a sender-side transport outcome: for example, “Transfer finished” or “FairDrop finished sending the file.” Keep receiver storage/opening as a browser-owned follow-up instruction, never an asserted fact. Rewrite Flow 1 step 9 as an expected user action, not product-observed state.

### TCH-05 — “Modern browser” and the primary iPhone journey have no support promise or validation floor

- **Severity:** High
- **Classification:** Product decision
- **Spine locations:** `EXPERIENCE.md` § **Foundation**, first paragraph; § **Information Architecture**, **Receiver browser/download UI**; § **Responsive & Platform**, **Receiver browser**; § **Key Flows**, Flow 1 steps 5–9; § **Inspiration & Anti-patterns**, **AirDrop north star**.
- **Evidence:** The UX makes Windows-to-iPhone the primary named journey and says the ZIP is available in iOS Files, while canonical acceptance only says “a modern browser on the same LAN” and native smoke requires a nearby browser. No supported receiver-browser/OS matrix, minimum versions, iPhone Safari acceptance scenario, attachment filename check, ZIP-open check, or browser-owned prompt expectation is defined.
- **Consequence:** “Any browser” or “any device” can leak into marketing and UI without evidence. A primary journey can fail on the exact platform combination used to sell the concept while still satisfying a generic desktop-browser smoke test.
- **Fix:** Decide a receiver compatibility promise and verification matrix. At minimum, test the primary Windows sender → current iPhone Safari journey and the Mac sender → Windows browser journey, including filename, single-use behavior, download prompt, ZIP validity/opening, and failure states. Until proven, use “a supported modern browser on the same local network,” not “any device.”

### TCH-06 — Copy URL is a sender-side action without a complete cross-device handoff instruction

- **Severity:** Medium
- **Classification:** Documentation/interaction defect
- **Spine locations:** `EXPERIENCE.md` § **Component Patterns**, **Direct URL Row** and **Copy Feedback**; § **Key Flows**, Flow 2 steps 3–4. `DESIGN.md` § **Components**, **Direct URL Row**.
- **Evidence:** Copying puts the URL on the sender’s clipboard; it does not move it to the receiving Windows PC, Mac, iPhone, or other browser. The spine neither says how the receiver obtains it nor distinguishes “copy for direct entry in the receiver browser” from posting it through a channel that can retain or preview it.
- **Consequence:** The fallback to QR is not operationally self-explanatory across ecosystems. Users may assume cross-device clipboard support, manually type an impractically long token, or paste the bearer URL into a persistent third-party service.
- **Fix:** Label the action “Copy download link” and require helper text: “Open this link in the receiving device’s browser.” Treat QR as the primary cross-device path. Pair this with TCH-02’s first-opener warning and avoid implying that FairDrop controls copies once the URL enters another app.

### TCH-07 — Different-network failure has no honest sender/receiver recovery model

- **Severity:** Medium
- **Classification:** Documentation defect; help placement is a product decision
- **Spine locations:** `EXPERIENCE.md` § **State Patterns**, **Offline/different LAN**; § **Trusted-LAN & Privacy**; § **Key Flows**, Flow 1 step 5.
- **Evidence:** The draft says stage/claim “fails safely,” but in the common case the sender can remain Staged while the receiving browser shows a generic connection failure. With no receiver page, FairDrop cannot explain the error on that device. “Same trusted network” also does not translate the common iPhone action: keep Wi-Fi on and use the same local network as the sender; guest/client-isolated networks may not work.
- **Consequence:** Users wait at two apparently valid surfaces, blame the QR code, or repeatedly scan a still-live link without a recovery cue.
- **Fix:** Require staged troubleshooting copy or a progressive help affordance: confirm both devices are on the same local network, note that guest/isolated networks may block device-to-device traffic, then Cancel and create a fresh link after correcting the network. Do not claim the sender can diagnose the receiver’s route.

### TCH-08 — No-cloud/no-copy truth is not yet a required visible statement and the sample is ambiguous

- **Severity:** Medium
- **Classification:** Documentation/copy defect
- **Spine locations:** `EXPERIENCE.md` § **Foundation**, second paragraph; § **Voice and Tone**, “FairDrop doesn’t keep a copy” and terminology rule; § **Component Patterns**, **Trusted-LAN Note**; § **Trusted-LAN & Privacy**, third bullet.
- **Evidence:** The implementation boundary is strong: no cloud, history, telemetry, payload archive, or persistent product state. The visible component contract requires only trusted-LAN/plain-HTTP disclosure, and the sample “FairDrop doesn’t keep a copy” can be misread as saying no copies exist, despite the original remaining on the sender and the download remaining on the receiver. The terminology rule also broadly bans “upload” and “cloud,” which can prevent an accurate negated disclosure.
- **Consequence:** Consumers may not learn the product’s strongest trust benefit, or may infer deletion/ephemerality beyond what FairDrop controls.
- **Fix:** Require one compact, literal statement such as “Sent directly over your local network. FairDrop does not upload or store an extra copy.” Explicitly say that the receiver keeps the downloaded file. Permit “cloud” and “upload” in truthful negated disclosures while continuing to ban claims of a cloud feature.

### TCH-09 — The AirDrop analogy suppresses real setup and browser friction

- **Severity:** Medium
- **Classification:** Documentation/positioning defect
- **Spine locations:** `EXPERIENCE.md` § **Inspiration & Anti-patterns**, **AirDrop north star** and **Reject setup rituals**; § **Foundation**; § **Key Flows**, Flow 1 climax. The UX memlog’s north-star and broader any-device intent increase the risk.
- **Evidence:** “Source and destination OS do not require configuration choices” sits beside required same-network setup, first-use firewall permission, QR/link opening, browser-owned download prompts, and ZIP opening. The UX does bound V1 elsewhere, but the aspirational language is broad enough to become “AirDrop for any device.”
- **Consequence:** Product and implementation copy can overpromise parity with native AirDrop discovery, identity, encryption, confirmation, and receiver completion.
- **Fix:** Recast the analogy as an internal friction benchmark only. Approve an external promise that names the actual mechanism: “Send from FairDrop on Windows or Mac to one browser on the same local network—no account or receiver app.” Explicitly ban “AirDrop for any device,” “works with every device,” and equivalent claims until supported.

### TCH-10 — Generic receiver HTTP failures are technically correct but not a consumer recovery experience

- **Severity:** Medium
- **Classification:** Product decision, not a spine defect in the protocol description
- **Spine locations:** `EXPERIENCE.md` § **Foundation**, no custom receiver page; § **Information Architecture**, **Receiver browser/download UI**; § **State Patterns**, receiver `404`, `423`, and `410` rows; § **Responsive & Platform**, **Receiver browser**.
- **Evidence:** The spine accurately carries the binding disguise/status contract: wrong method/route/token is generic `404`, a competing valid request is `423`, and a source changed before headers is `410`. An ordinary browser may render only terse status text, and no FairDrop page can explain “wrong/expired link,” “another receiver got there first,” or “ask the sender to retry.”
- **Consequence:** Receiver confusion is an accepted V1 limitation unless the sender surface supplies recovery. Marketing cannot promise a guided browser receiver experience.
- **Fix:** Choose explicitly between: (a) retain generic browser failures and add sender-side troubleshooting/new-link guidance, documenting this limitation; or (b) authorize an architecture/product change for safe branded receiver error pages without leaking route/token validity. Do not invent friendly receiver pages inside the UX spine while the current contract remains binding.

### TCH-11 — Platform arrows can still be read as native any-direction support

- **Severity:** Medium
- **Classification:** Documentation defect
- **Spine locations:** `EXPERIENCE.md` § **Responsive & Platform**, **Roadmap** row; § **Foundation**, second paragraph; § **Key Flows**, Flow 2 title.
- **Evidence:** Foundation correctly says native iOS/Android sending is roadmap-only, but “Mac→Windows” and “Windows→Mac use the same V1 model” omit the role labels at the point of comparison. The arrows can be read as native FairDrop at both ends, especially beside the AirDrop analogy.
- **Consequence:** iPhone→Windows, Windows→Mac, or Mac→Windows may be interpreted as symmetric native sending/receiving rather than desktop sender → ordinary browser receiver.
- **Fix:** Spell out every matrix example: “Mac FairDrop sender → Windows browser receiver” and “Windows FairDrop sender → Mac browser receiver.” Add “iPhone is receiver-only in V1; iPhone sending is roadmap-only” wherever the platform matrix is summarized.

### TCH-12 — Linked working boards contain unsafe copy and visuals despite the precedence disclaimer

- **Severity:** Low
- **Classification:** Working-artifact hygiene defect
- **Spine locations:** `DESIGN.md` opening audit-reference note; § **Colors**, **QR**; § **Components**, **Trusted-LAN Note**; § **Do's and Don'ts**, QR and trusted-LAN rows. `EXPERIENCE.md` opening audit-reference note and § **Voice and Tone**.
- **Evidence:** The selected Paper Relay / Terracotta working examples use a green “TRUSTED LAN” success pill, “nothing stored,” a staged progress bar, shortened vanity URLs, and—in the design-direction board—a rotated QR with a center overlay. Other visible theme candidates include “Copy secure link” and “Nothing leaves your network.” The final spines correctly prohibit these, but they link the boards as audit references and rely on a precedence sentence to neutralize visually memorable examples.
- **Consequence:** An implementer or stakeholder can lift the mockup instead of the contract, reintroducing security theater, false QR rendering, staged progress fiction, or overbroad privacy copy.
- **Fix:** Keep the artifacts non-normative, but add a conspicuous “exploration only—copy and QR treatment are not approved” banner or annotate the unsafe examples. The spines should remain the sole source for production copy and behavior.

## Product decisions to close

These are decisions, not mere editorial repairs:

1. **Firewall guidance and denial recovery (High):** placement, Windows private/public advice, macOS wording, and recovery instructions.
2. **Receiver support promise (High):** which browser/OS combinations are supported and what native smoke/compatibility evidence is required, especially Windows sender → iPhone Safari.
3. **Bearer-link preview behavior (High):** whether a third-party `GET` prefetch claiming the one-shot transfer is an accepted limitation or requires protocol/product hardening.
4. **Generic receiver failures (Medium):** accept browser-native `404`/`423`/`410` plus sender-side help, or authorize a safe receiver-page architecture change.
5. **Cross-device URL fallback guidance (Medium):** approved instructions and whether any channels are recommended for moving the bearer link between devices.

## Confirmed strengths

- **V1 role boundary is explicit:** `EXPERIENCE.md` § **Foundation** correctly fixes Windows/macOS native sender → same-LAN browser receiver and makes native mobile sending roadmap-only.
- **Folder semantics are honest:** § **Component Patterns**, **Item Summary** and **Progress Meter**, plus Flow 1, clearly say folders download as ZIPs and never fake a ZIP percentage from logical size.
- **Persistence claims are technically grounded:** § **Foundation** and § **Trusted-LAN & Privacy** match canonical no-database/no-cloud/no-history/no-payload-archive constraints.
- **Security theater is actively rejected:** `DESIGN.md` § **Do's and Don'ts** and `EXPERIENCE.md` § **Inspiration & Anti-patterns** prohibit locks, shields, and unsupported secure/private/encrypted claims.
- **Receiver ambiguity is represented in the state contract:** the first-claim and competing-claim rows are accurate; the missing work is translating them into consumer-level meaning.

## Release recommendation

Resolve TCH-03 and TCH-05 as explicit product decisions, then revise the spines for TCH-01, TCH-02, and TCH-04 before changing status from draft. The medium findings can be closed in the same copy pass. No lifecycle or architecture change is required unless the team chooses link-preview hardening or branded receiver error pages.
