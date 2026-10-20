; SPDX-License-Identifier: GPL-3.0-only
;
; installer.nsi - Installeur Windows de SVG Print Layout.
;
; Appelé par build-windows.sh, qui copie ce fichier dans son dossier de travail
; avec deux fichiers générés, puis lance makensis dessus :
;   build-vars.nsh        les valeurs de cette construction (!define) :
;                           APP_VERSION          version affichée (0.1.0-dev)
;                           APP_VERSION_NUMERIC  version à quatre nombres (0.1.0.0)
;                           STAGE_DIR            dossier du programme à installer
;                           OUTPUT_FILE          installeur à produire
;                           APP_ICON             icône (.ico)
;   uninstall-files.nsh   la liste exacte des fichiers à supprimer (Delete,
;                         RMDir) : le désinstalleur n'efface jamais un dossier
;                         entier, au cas où l'utilisateur aurait choisi un
;                         dossier d'installation qui contient autre chose.
;
; Ce fichier est en UTF-8 avec marque d'ordre des octets (BOM), que NSIS exige
; pour lire les accents ; ne pas l'enregistrer sans BOM.

Unicode true

!include "build-vars.nsh"

Name "SVG Print Layout"
OutFile "${OUTPUT_FILE}"
InstallDir "$PROGRAMFILES64\SVG Print Layout"
RequestExecutionLevel admin
SetCompressor /SOLID lzma
BrandingText "SVG Print Layout ${APP_VERSION}"

!include "MUI2.nsh"
!include "FileFunc.nsh"  ; ${GetSize}, pour EstimatedSize
!include "x64.nsh"       ; ${RunningX64}

!define MUI_ABORTWARNING
!define MUI_ICON "${APP_ICON}"
!define MUI_UNICON "${APP_ICON}"
; Tant que le programme n'a pas d'interface graphique, il n'y a rien à lancer à
; la fin de l'installation : ajouter ici MUI_FINISHPAGE_RUN le moment venu.

!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH

!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES

!insertmacro MUI_LANGUAGE "French"

VIProductVersion "${APP_VERSION_NUMERIC}"
VIAddVersionKey /LANG=${LANG_FRENCH} "ProductName" "SVG Print Layout"
VIAddVersionKey /LANG=${LANG_FRENCH} "ProductVersion" "${APP_VERSION}"
VIAddVersionKey /LANG=${LANG_FRENCH} "CompanyName" "Gilles MAGNEVILLE"
VIAddVersionKey /LANG=${LANG_FRENCH} "LegalCopyright" "GNU GPL v3"
VIAddVersionKey /LANG=${LANG_FRENCH} "FileDescription" "Installeur de SVG Print Layout"
VIAddVersionKey /LANG=${LANG_FRENCH} "FileVersion" "${APP_VERSION}"

!define UNINSTALL_KEY "Software\Microsoft\Windows\CurrentVersion\Uninstall\SVGPrintLayout"
!define APP_KEY "Software\SVGPrintLayout"

Function .onInit
    ${IfNot} ${RunningX64}
        MessageBox MB_OK|MB_ICONSTOP "SVG Print Layout nécessite Windows 64 bits (x64)."
        Abort
    ${EndIf}
    ; Écrire dans la base de registre 64 bits, et non dans la vue 32 bits de
    ; cet installeur.
    SetRegView 64
    SetShellVarContext all

    ; Mise à jour : on propose le dossier de l'installation précédente, sauf si
    ; l'utilisateur a donné le sien (/D=...), qui est alors déjà dans $INSTDIR.
    StrCmp $INSTDIR "$PROGRAMFILES64\SVG Print Layout" 0 instdir_done
    ReadRegStr $0 HKLM "${APP_KEY}" "InstallDir"
    StrCmp $0 "" instdir_done 0
    StrCpy $INSTDIR $0
    instdir_done:
FunctionEnd

Function un.onInit
    SetRegView 64
    SetShellVarContext all
FunctionEnd

Section "SVG Print Layout (obligatoire)" SecMain
    SectionIn RO

    ; Mise à jour : on retire d'abord la version installée, pour qu'il ne reste
    ; aucun fichier de l'ancienne (une DLL périmée, par exemple).
    IfFileExists "$INSTDIR\Uninstall.exe" 0 fresh
        ExecWait '"$INSTDIR\Uninstall.exe" /S _?=$INSTDIR'
        Delete "$INSTDIR\Uninstall.exe"
    fresh:

    SetOutPath "$INSTDIR"
    File /r "${STAGE_DIR}\*.*"

    WriteRegStr HKLM "${APP_KEY}" "InstallDir" "$INSTDIR"
    WriteRegStr HKLM "${UNINSTALL_KEY}" "DisplayName" "SVG Print Layout"
    WriteRegStr HKLM "${UNINSTALL_KEY}" "DisplayVersion" "${APP_VERSION}"
    WriteRegStr HKLM "${UNINSTALL_KEY}" "Publisher" "Gilles MAGNEVILLE"
    WriteRegStr HKLM "${UNINSTALL_KEY}" "URLInfoAbout" "https://github.com/gillesmagneville/svg-print-layout"
    WriteRegStr HKLM "${UNINSTALL_KEY}" "DisplayIcon" "$INSTDIR\bin\svgprintlayout.exe"
    WriteRegStr HKLM "${UNINSTALL_KEY}" "InstallLocation" "$INSTDIR"
    WriteRegStr HKLM "${UNINSTALL_KEY}" "UninstallString" '"$INSTDIR\Uninstall.exe"'
    WriteRegStr HKLM "${UNINSTALL_KEY}" "QuietUninstallString" '"$INSTDIR\Uninstall.exe" /S'
    WriteRegDWORD HKLM "${UNINSTALL_KEY}" "NoModify" 1
    WriteRegDWORD HKLM "${UNINSTALL_KEY}" "NoRepair" 1

    ; Taille installée, affichée dans Paramètres > Applications. À calculer une
    ; fois les fichiers copiés. IntFmt demande un format de type printf : avec
    ; "0xX" il ne convertit rien et la taille retombe à 0 sans erreur.
    ${GetSize} "$INSTDIR" "/S=0K" $0 $1 $2
    IntFmt $0 "0x%X" $0
    WriteRegDWORD HKLM "${UNINSTALL_KEY}" "EstimatedSize" "$0"

    WriteUninstaller "$INSTDIR\Uninstall.exe"
SectionEnd

Section "Raccourci du menu Démarrer" SecStartMenu
    CreateDirectory "$SMPROGRAMS\SVG Print Layout"
    CreateShortCut "$SMPROGRAMS\SVG Print Layout\SVG Print Layout.lnk" "$INSTDIR\bin\svgprintlayout.exe"
    CreateShortCut "$SMPROGRAMS\SVG Print Layout\Désinstaller.lnk" "$INSTDIR\Uninstall.exe"
SectionEnd

Section /o "Raccourci sur le Bureau" SecDesktop
    CreateShortCut "$DESKTOP\SVG Print Layout.lnk" "$INSTDIR\bin\svgprintlayout.exe"
SectionEnd

Section "Uninstall"
    !include "uninstall-files.nsh"
    Delete "$INSTDIR\Uninstall.exe"
    RMDir "$INSTDIR"

    Delete "$SMPROGRAMS\SVG Print Layout\SVG Print Layout.lnk"
    Delete "$SMPROGRAMS\SVG Print Layout\Désinstaller.lnk"
    RMDir "$SMPROGRAMS\SVG Print Layout"
    Delete "$DESKTOP\SVG Print Layout.lnk"

    DeleteRegKey HKLM "${UNINSTALL_KEY}"
    DeleteRegKey HKLM "${APP_KEY}"
SectionEnd
