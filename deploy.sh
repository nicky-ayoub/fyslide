#!/usr/bin/env bash
# deploy.sh - deploy bin artifacts with a single SSH connection
set -euo pipefail

# Remote user/host
REMOTE="tester@alien"

# Files to deploy (relative to repo root)
FILES=(bin/fyslide bin/fyslide-cli)

# Create a tar stream of the files and send it over one SSH session.
# On the remote side we first back up any existing targets, then extract
# the streamed tar into the remote home directory (so paths like
# "bin/fyslide" are created/overwritten in $HOME).


# Stream the files and run remote backup + extract in one SSH session.
tar -C . -cf - "${FILES[@]}" | ssh ${REMOTE} 'set -e; \
	# Ensure Fyne cache directory exists so the app can create its cache.
	mkdir -p "$HOME/.cache/fyne/com.github.nicky-ayoub/fyslide"; \
	chmod 700 "$HOME/.cache/fyne" || true; \
	cp -a bin/fyslide bin/fyslide2.backup 2>/dev/null || true; \
	cp -a bin/fyslide-cli bin/fyslide-cli2.backup 2>/dev/null || true; \
	tar -C "$HOME" -xf -'

echo "Deploy complete to ${REMOTE}"
