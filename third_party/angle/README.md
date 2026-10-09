# ANGLE

64-bit `libEGL.dll` and `libGLESv2.dll` that the Zone Builder loads at run time (OpenGL ES 3.0 over Direct3D 11). `scripts/build.ps1` copies them next to `bin/Zone Builder.exe`. License: BSD, in `LICENSE`.

- Version: ANGLE 2.1.23105, git hash 5d4df51d1d7d (as the app logs it at startup).
- Taken from a Chromium Embedded Framework build (`cef.win64`); they depend only on Windows system DLLs (`d3dcompiler_47.dll` comes from System32).
- Measured with these files: `GL_EXT_clip_control` (docs/adr/0001), DXT1/3/5 + sRGB and anisotropic filtering (docs/adr/0002).

| File | SHA-256 |
| --- | --- |
| `libEGL.dll` | `f08eb4443498e35a527ef8cac4ab835f801538a019985f3d75043f784ac4329a` |
| `libGLESv2.dll` | `ed8394fa69f8f816e0b392353c178680e0a221fefd0b54c58a61af033fdf9b1f` |

To swap the build, replace both files with a 64-bit ANGLE that exposes the extensions above (the app refuses to start without them), update the table and run the app once.
