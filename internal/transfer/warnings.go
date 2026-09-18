package transfer

// This file holds the two Staged warning constructors. A warning is not a
// failure: it travels beside a transfer that still works, so each one carries
// fixed registry copy and a stable code, and neither the offending entry names
// nor their count leave the process (AD-9).

// unportableNamesWarning is the Staged warning for a folder holding entries a
// Windows receiver cannot save. Fixed registry copy, like every warning: the
// number of offending entries and their names stay inside the process.
func unportableNamesWarning() Warning {
	public := PublicErrorOf(NewError(ErrNameWarning, "selection holds names a Windows receiver cannot save"))
	return Warning{Code: WarnUnportableNames, Message: public.Message}
}

// beaconWarning is the fixed non-fatal warning for a discovery failure. Its
// copy comes from the public registry rather than from the adapter, so no
// adapter text can reach the UI through it, and its code is constrained to the
// WarningCode type so the frontend's parser cannot meet one it does not
// recognise (Epic 1 retrospective item 3).
//
// This comment sat above unportableNamesWarning between Story 3.11 and the
// Epic 3 retrospective, which inserted that function between the comment and
// the function it describes. `go doc` printed it under the wrong name and
// printed nothing under this one (A7).
func beaconWarning() Warning {
	public := PublicErrorOf(NewError(ErrBeaconWarning, "device discovery is unavailable"))
	return Warning{Code: WarnBeaconUnavailable, Message: public.Message}
}
