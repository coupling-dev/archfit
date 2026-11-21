#!/bin/sh
# check-names.sh — fail when a tracked file names the old home of archfit.
#
# Usage: sh scripts/check-names.sh
#
# The project moved to github.com/coupling-dev and restarted at 0.9.0 (beta
# plan, docs/plans/20261120-archfit-0-9-new-home-and-beta.md, section 3). An
# old repository, image, tap or /vN module path in a current file sends a user
# to a release that is no longer maintained. People's logins are not matched.
#
# Excluded: history (docs/archived, docs/plans).
set -eu

# No \b: git grep -E on macOS ignores it, so a /vN path went unmatched there.
pattern='alexei-led/(archfit|homebrew-tap|tap/archfit)|ghcr\.io/alexei-led|archfit/v[0-9]+([^0-9A-Za-z_]|$)'

if git grep -n -I -E "$pattern" -- \
	':!docs/archived/**' \
	':!docs/plans/**' \
	':!scripts/check-names.sh'; then
	echo "check-names: the lines above name the old home; use the names in section 3 of the beta plan" >&2
	exit 1
fi
