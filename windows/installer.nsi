; SPDX-License-Identifier: GPL-3.0-only
;
; installer.nsi - Windows installer of Image Print Layout.
;
; Called by build-windows.sh, which copies this file into its working folder
; with two generated files, then runs makensis on it:
;   build-vars.nsh        the values of this build (!define):
;                           APP_VERSION          displayed version (0.1.0-dev)
;                           APP_VERSION_NUMERIC  four-number version (0.1.0.0)
;                           STAGE_DIR            folder of the program to install
;                           OUTPUT_FILE          installer to produce
;                           APP_ICON             icon (.ico)
;   uninstall-files.nsh   the exact list of the files to delete (Delete,
;                         RMDir): the uninstaller never erases a whole
;                         folder, in case the user chose an install
;                         folder that contains something else.
;
; The installer speaks the four languages of the program. NSIS picks that of
; Windows; English, declared first, is used when Windows is in another
; language. The uninstaller picks the same way: its shortcuts are therefore
; deleted under their four possible names.
;
; This file is UTF-8 with a byte order mark (BOM), which NSIS requires
; to read accents; do not save it without a BOM.

Unicode true

!include "build-vars.nsh"

Name "Image Print Layout"
OutFile "${OUTPUT_FILE}"
InstallDir "$PROGRAMFILES64\Image Print Layout"
RequestExecutionLevel admin
SetCompressor /SOLID lzma
BrandingText "Image Print Layout ${APP_VERSION}"

!include "MUI2.nsh"
!include "FileFunc.nsh"  ; ${GetSize}, for EstimatedSize
!include "x64.nsh"       ; ${RunningX64}

!define MUI_ABORTWARNING
!define MUI_ICON "${APP_ICON}"
!define MUI_UNICON "${APP_ICON}"
; No "Run the program" box at the end: the installer runs as
; administrator, and the program started from here would too. The Start
; menu shortcut starts it with the person's rights.

!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH

!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES

; English first: it is the fallback language.
!insertmacro MUI_LANGUAGE "English"
!insertmacro MUI_LANGUAGE "French"
!insertmacro MUI_LANGUAGE "Spanish"
!insertmacro MUI_LANGUAGE "German"

; Texts of the installer, in each language.
LangString MSG_NEEDS_X64 ${LANG_ENGLISH} "Image Print Layout requires 64-bit Windows (x64)."
LangString MSG_NEEDS_X64 ${LANG_FRENCH}  "Image Print Layout nécessite Windows 64 bits (x64)."
LangString MSG_NEEDS_X64 ${LANG_SPANISH} "Image Print Layout necesita Windows de 64 bits (x64)."
LangString MSG_NEEDS_X64 ${LANG_GERMAN}  "Image Print Layout benötigt 64-Bit-Windows (x64)."

LangString SEC_MAIN ${LANG_ENGLISH} "Image Print Layout (required)"
LangString SEC_MAIN ${LANG_FRENCH}  "Image Print Layout (obligatoire)"
LangString SEC_MAIN ${LANG_SPANISH} "Image Print Layout (obligatorio)"
LangString SEC_MAIN ${LANG_GERMAN}  "Image Print Layout (erforderlich)"

LangString SEC_START_MENU ${LANG_ENGLISH} "Start Menu shortcut"
LangString SEC_START_MENU ${LANG_FRENCH}  "Raccourci du menu Démarrer"
LangString SEC_START_MENU ${LANG_SPANISH} "Acceso directo del menú Inicio"
LangString SEC_START_MENU ${LANG_GERMAN}  "Verknüpfung im Startmenü"

LangString SEC_DESKTOP ${LANG_ENGLISH} "Desktop shortcut"
LangString SEC_DESKTOP ${LANG_FRENCH}  "Raccourci sur le Bureau"
LangString SEC_DESKTOP ${LANG_SPANISH} "Acceso directo en el Escritorio"
LangString SEC_DESKTOP ${LANG_GERMAN}  "Verknüpfung auf dem Desktop"

LangString LNK_UNINSTALL ${LANG_ENGLISH} "Uninstall"
LangString LNK_UNINSTALL ${LANG_FRENCH}  "Désinstaller"
LangString LNK_UNINSTALL ${LANG_SPANISH} "Desinstalar"
LangString LNK_UNINSTALL ${LANG_GERMAN}  "Deinstallieren"

VIProductVersion "${APP_VERSION_NUMERIC}"
!macro VersionKeys LANG DESCRIPTION
    VIAddVersionKey /LANG=${LANG} "ProductName" "Image Print Layout"
    VIAddVersionKey /LANG=${LANG} "ProductVersion" "${APP_VERSION}"
    VIAddVersionKey /LANG=${LANG} "CompanyName" "Gilles MAGNEVILLE"
    VIAddVersionKey /LANG=${LANG} "LegalCopyright" "Gilles MAGNEVILLE - GNU GPL v3"
    VIAddVersionKey /LANG=${LANG} "FileDescription" "${DESCRIPTION}"
    VIAddVersionKey /LANG=${LANG} "FileVersion" "${APP_VERSION}"
!macroend
!insertmacro VersionKeys ${LANG_ENGLISH} "Image Print Layout installer"
!insertmacro VersionKeys ${LANG_FRENCH}  "Installeur d'Image Print Layout"
!insertmacro VersionKeys ${LANG_SPANISH} "Instalador de Image Print Layout"
!insertmacro VersionKeys ${LANG_GERMAN}  "Installationsprogramm für Image Print Layout"

!define UNINSTALL_KEY "Software\Microsoft\Windows\CurrentVersion\Uninstall\ImagePrintLayout"
!define APP_KEY "Software\ImagePrintLayout"
; The program was called "SVG Print Layout" up to version 1.0.x: its
; registry keys, which the installer removes (see SecMain).
!define OLD_APP_KEY "Software\SVGPrintLayout"

Function .onInit
    ${IfNot} ${RunningX64}
        MessageBox MB_OK|MB_ICONSTOP "$(MSG_NEEDS_X64)"
        Abort
    ${EndIf}
    ; Write to the 64-bit registry, not to the 32-bit view of
    ; this installer.
    SetRegView 64
    SetShellVarContext all

    ; Update: we offer the folder of the previous installation, unless
    ; the user gave their own (/D=...), which is then already in $INSTDIR.
    StrCmp $INSTDIR "$PROGRAMFILES64\Image Print Layout" 0 instdir_done
    ReadRegStr $0 HKLM "${APP_KEY}" "InstallDir"
    StrCmp $0 "" instdir_done 0
    StrCpy $INSTDIR $0
    instdir_done:
FunctionEnd

Function un.onInit
    SetRegView 64
    SetShellVarContext all
FunctionEnd

Section "$(SEC_MAIN)" SecMain
    SectionIn RO

    ; Renaming: an installation of "SVG Print Layout" (up to version
    ; 1.0.x) is uninstalled by its own uninstaller, which removes its
    ; files, its shortcuts and its keys. The user's settings and projects
    ; are not in its folder: they stay.
    ReadRegStr $0 HKLM "${OLD_APP_KEY}" "InstallDir"
    StrCmp $0 "" old_done 0
    IfFileExists "$0\Uninstall.exe" 0 old_done
        ExecWait '"$0\Uninstall.exe" /S _?=$0'
        Delete "$0\Uninstall.exe"
        RMDir "$0"
    old_done:

    ; Update: we first remove the installed version, so that no file
    ; of the old one remains (an outdated DLL, for example).
    IfFileExists "$INSTDIR\Uninstall.exe" 0 fresh
        ExecWait '"$INSTDIR\Uninstall.exe" /S _?=$INSTDIR'
        Delete "$INSTDIR\Uninstall.exe"
    fresh:

    SetOutPath "$INSTDIR"
    File /r "${STAGE_DIR}\*.*"

    WriteRegStr HKLM "${APP_KEY}" "InstallDir" "$INSTDIR"
    WriteRegStr HKLM "${UNINSTALL_KEY}" "DisplayName" "Image Print Layout"
    WriteRegStr HKLM "${UNINSTALL_KEY}" "DisplayVersion" "${APP_VERSION}"
    WriteRegStr HKLM "${UNINSTALL_KEY}" "Publisher" "Gilles MAGNEVILLE"
    WriteRegStr HKLM "${UNINSTALL_KEY}" "URLInfoAbout" "https://github.com/gillesmagneville/image-print-layout"
    WriteRegStr HKLM "${UNINSTALL_KEY}" "DisplayIcon" "$INSTDIR\bin\imageprintlayout.exe"
    WriteRegStr HKLM "${UNINSTALL_KEY}" "InstallLocation" "$INSTDIR"
    WriteRegStr HKLM "${UNINSTALL_KEY}" "UninstallString" '"$INSTDIR\Uninstall.exe"'
    WriteRegStr HKLM "${UNINSTALL_KEY}" "QuietUninstallString" '"$INSTDIR\Uninstall.exe" /S'
    WriteRegDWORD HKLM "${UNINSTALL_KEY}" "NoModify" 1
    WriteRegDWORD HKLM "${UNINSTALL_KEY}" "NoRepair" 1

    ; Installed size, shown in Settings > Apps. To be computed once
    ; the files are copied. IntFmt wants a printf-type format: with
    ; "0xX" it converts nothing and the size falls back to 0 without error.
    ${GetSize} "$INSTDIR" "/S=0K" $0 $1 $2
    IntFmt $0 "0x%X" $0
    WriteRegDWORD HKLM "${UNINSTALL_KEY}" "EstimatedSize" "$0"

    WriteUninstaller "$INSTDIR\Uninstall.exe"
SectionEnd

Section "$(SEC_START_MENU)" SecStartMenu
    CreateDirectory "$SMPROGRAMS\Image Print Layout"
    CreateShortCut "$SMPROGRAMS\Image Print Layout\Image Print Layout.lnk" "$INSTDIR\bin\imageprintlayout.exe"
    CreateShortCut "$SMPROGRAMS\Image Print Layout\$(LNK_UNINSTALL).lnk" "$INSTDIR\Uninstall.exe"
SectionEnd

Section /o "$(SEC_DESKTOP)" SecDesktop
    CreateShortCut "$DESKTOP\Image Print Layout.lnk" "$INSTDIR\bin\imageprintlayout.exe"
SectionEnd

Section "Uninstall"
    !include "uninstall-files.nsh"
    Delete "$INSTDIR\Uninstall.exe"
    RMDir "$INSTDIR"

    Delete "$SMPROGRAMS\Image Print Layout\Image Print Layout.lnk"
    ; The uninstaller shortcut bears the name of the language of the installation,
    ; perhaps not that of Windows today: we delete all four.
    Delete "$SMPROGRAMS\Image Print Layout\Uninstall.lnk"
    Delete "$SMPROGRAMS\Image Print Layout\Désinstaller.lnk"
    Delete "$SMPROGRAMS\Image Print Layout\Desinstalar.lnk"
    Delete "$SMPROGRAMS\Image Print Layout\Deinstallieren.lnk"
    RMDir "$SMPROGRAMS\Image Print Layout"
    Delete "$DESKTOP\Image Print Layout.lnk"

    DeleteRegKey HKLM "${UNINSTALL_KEY}"
    DeleteRegKey HKLM "${APP_KEY}"
SectionEnd
