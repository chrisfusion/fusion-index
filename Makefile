# SPDX-License-Identifier: GPL-3.0-or-later

.PHONY: vendor check-vendor

## Refresh vendor/ from go.mod (committed so builds work offline).
vendor:
	go mod tidy
	go mod vendor

## Fail when vendor/ drifted from go.mod/go.sum.
check-vendor:
	go mod vendor
	git diff --exit-code -- vendor go.mod go.sum
	@test -z "$$(git ls-files --others --exclude-standard -- vendor)" || (echo "untracked files in vendor/" && exit 1)
