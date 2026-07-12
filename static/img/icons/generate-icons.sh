#!/usr/bin/env sh
# Regenerate the favicon / app-icon sizes from app-icon.png.
#
# Save the full-size source icon as app-icon.png in this directory, then run:
#   sh generate-icons.sh
#
# Requires ffmpeg on PATH (handles PNG transparency). Lanczos scaling keeps the
# downscaled icons sharp.
set -e

cd "$(dirname "$0")"

src="app-icon.png"
if [ ! -f "$src" ]; then
	echo "error: $src not found in $(pwd)" >&2
	echo "Save the source icon as app-icon.png here first." >&2
	exit 1
fi

for s in 16 32 48 180 192 256 512; do
	ffmpeg -y -loglevel error -i "$src" -vf "scale=${s}:${s}:flags=lanczos" "icon-${s}x${s}.png"
	echo "wrote icon-${s}x${s}.png"
done

echo "done"
