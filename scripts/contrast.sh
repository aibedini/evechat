#!/usr/bin/env bash
# Contrast audit for the design tokens used by the dashboard, widget and setup
# wizard. Fails CI when any text/background pair drops below WCAG AA.
set -euo pipefail
node "$(dirname "$0")/contrast-check.mjs"
