#!/bin/bash
set -euo pipefail
version="$1"
arch="$2"
base="dist/ostrm_${version}_darwin_${arch}"
stage="dist/pkg-stage-${arch}"
mkdir -p "$stage/Applications/OStrm.app/Contents/MacOS" "$stage/Applications/OStrm.app/Contents/Resources"
cp "$base/ostrm" "$stage/Applications/OStrm.app/Contents/MacOS/ostrm"
cp packaging/macos/Info.plist "$stage/Applications/OStrm.app/Contents/Info.plist"
cp LICENSE "$stage/Applications/OStrm.app/Contents/Resources/LICENSE"
/usr/libexec/PlistBuddy -c "Set :CFBundleVersion ${version%%-*}" "$stage/Applications/OStrm.app/Contents/Info.plist"
if [ -n "${APPLE_APPLICATION_IDENTITY:-}" ]; then codesign --force --options runtime --timestamp --sign "$APPLE_APPLICATION_IDENTITY" "$stage/Applications/OStrm.app"; fi
pkgbuild --root "$stage" --identifier io.github.moersity.ostrm --version "${version%%-*}" --install-location / "${base}.pkg"
if [ -n "${APPLE_INSTALLER_IDENTITY:-}" ]; then productsign --sign "$APPLE_INSTALLER_IDENTITY" "${base}.pkg" "${base}.signed.pkg"; mv "${base}.signed.pkg" "${base}.pkg"; fi
if [ -n "${APPLE_NOTARY_PROFILE:-}" ]; then xcrun notarytool submit "${base}.pkg" --keychain-profile "$APPLE_NOTARY_PROFILE" --wait; xcrun stapler staple "${base}.pkg"; fi
