# build/

Wails packaging artifacts. Files here are picked up by `wails build` per target platform.

## Layout

- `appicon.png` 1024x1024 PNG used for the app icon. Add one before shipping production builds.
- `windows/` icons, manifest, NSIS installer config, wix templates.
- `darwin/` Info.plist customizations, entitlements, dmg background.
- `linux/` desktop entry, AppImage metadata.

Generated artifacts land in `build/bin/` and are gitignored.
