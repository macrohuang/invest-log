# -*- mode: python ; coding: utf-8 -*-
"""
PyInstaller spec file for Invest Log backend sidecar.

Build with:
    pyinstaller invest-log-backend.spec

Output will be in dist/invest-log-backend
"""

import sys
from pathlib import Path

block_cipher = None

# Determine platform-specific naming for Tauri sidecar
if sys.platform == 'darwin':
    import platform
    arch = platform.machine()
    if arch == 'arm64':
        binary_name = 'invest-log-backend-aarch64-apple-darwin'
    else:
        binary_name = 'invest-log-backend-x86_64-apple-darwin'
elif sys.platform == 'win32':
    binary_name = 'invest-log-backend-x86_64-pc-windows-msvc'
else:
    binary_name = 'invest-log-backend-x86_64-unknown-linux-gnu'

a = Analysis(
    ['app.py'],
    pathex=[],
    binaries=[],
    datas=[
        ('templates', 'templates'),
        ('static', 'static'),
    ],
    hiddenimports=[
        'uvicorn.logging',
        'uvicorn.loops',
        'uvicorn.loops.auto',
        'uvicorn.protocols',
        'uvicorn.protocols.http',
        'uvicorn.protocols.http.auto',
        'uvicorn.protocols.websockets',
        'uvicorn.protocols.websockets.auto',
        'uvicorn.lifespan',
        'uvicorn.lifespan.on',
        'uvicorn.lifespan.off',
        'fastapi',
        'starlette',
        'jinja2',
        'akshare',
        'pandas',
        'numpy',
    ],
    hookspath=[],
    hooksconfig={},
    runtime_hooks=[],
    excludes=[],
    win_no_prefer_redirects=False,
    win_private_assemblies=False,
    cipher=block_cipher,
    noarchive=False,
)

pyz = PYZ(a.pure, a.zipped_data, cipher=block_cipher)

exe = EXE(
    pyz,
    a.scripts,
    a.binaries,
    a.zipfiles,
    a.datas,
    [],
    name=binary_name,
    debug=False,
    bootloader_ignore_signals=False,
    strip=False,
    upx=True,
    upx_exclude=[],
    runtime_tmpdir=None,
    console=True,  # Keep console for logging; set to False for production
    disable_windowed_traceback=False,
    argv_emulation=False,
    target_arch=None,
    codesign_identity=None,
    entitlements_file=None,
)
