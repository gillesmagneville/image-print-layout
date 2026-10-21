; SPDX-License-Identifier: GPL-3.0-only
;
; installer.nsi - Installeur Windows d'Image Print Layout.
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
; L'installeur parle les quatre langues du programme. NSIS choisit celle de
; Windows ; l'anglais, déclaré en premier, sert quand Windows est dans une autre
; langue. Le désinstalleur choisit de la même façon : ses raccourcis sont donc
; supprimés sous leurs quatre noms possibles.
;
; Ce fichier est en UTF-8 avec marque d'ordre des octets (BOM), que NSIS exige
; pour lire les accents ; ne pas l'enregistrer sans BOM.

Unicode true

!include "build-vars.nsh"

Name "Image Print Layout"
OutFile "${OUTPUT_FILE}"
InstallDir "$PROGRAMFILES64\Image Print Layout"
RequestExecutionLevel admin
SetCompressor /SOLID lzma
BrandingText "Image Print Layout ${APP_VERSION}"

!include "MUI2.nsh"
!include "FileFunc.nsh"  ; ${GetSize}, pour EstimatedSize
!include "x64.nsh"       ; ${RunningX64}

!define MUI_ABORTWARNING
!define MUI_ICON "${APP_ICON}"
!define MUI_UNICON "${APP_ICON}"
; Pas de case « Lancer le programme » à la fin : l'installeur s'exécute en
; administrateur, et le programme lancé d'ici le serait aussi. Le raccourci du
; menu Démarrer le lance avec les droits de la personne.

!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH

!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES

; L'anglais d'abord : c'est la langue de repli.
!insertmacro MUI_LANGUAGE "English"
!insertmacro MUI_LANGUAGE "French"
!insertmacro MUI_LANGUAGE "Spanish"
!insertmacro MUI_LANGUAGE "German"

; Textes de l'installeur, dans chaque langue.
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
; Le programme s'appelait « SVG Print Layout » jusqu'à la version 1.0.x : ses
; clés de registre, que l'installeur retire (voir SecMain).
!define OLD_APP_KEY "Software\SVGPrintLayout"

Function .onInit
    ${IfNot} ${RunningX64}
        MessageBox MB_OK|MB_ICONSTOP "$(MSG_NEEDS_X64)"
        Abort
    ${EndIf}
    ; Écrire dans la base de registre 64 bits, et non dans la vue 32 bits de
    ; cet installeur.
    SetRegView 64
    SetShellVarContext all

    ; Mise à jour : on propose le dossier de l'installation précédente, sauf si
    ; l'utilisateur a donné le sien (/D=...), qui est alors déjà dans $INSTDIR.
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

    ; Renommage : une installation de « SVG Print Layout » (jusqu'à la version
    ; 1.0.x) est désinstallée par son propre désinstalleur, qui retire ses
    ; fichiers, ses raccourcis et ses clés. Les réglages et les projets de
    ; l'utilisateur ne sont pas dans son dossier : ils restent.
    ReadRegStr $0 HKLM "${OLD_APP_KEY}" "InstallDir"
    StrCmp $0 "" old_done 0
    IfFileExists "$0\Uninstall.exe" 0 old_done
        ExecWait '"$0\Uninstall.exe" /S _?=$0'
        Delete "$0\Uninstall.exe"
        RMDir "$0"
    old_done:

    ; Mise à jour : on retire d'abord la version installée, pour qu'il ne reste
    ; aucun fichier de l'ancienne (une DLL périmée, par exemple).
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

    ; Taille installée, affichée dans Paramètres > Applications. À calculer une
    ; fois les fichiers copiés. IntFmt demande un format de type printf : avec
    ; "0xX" il ne convertit rien et la taille retombe à 0 sans erreur.
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
    ; Le raccourci du désinstalleur porte le nom de la langue de l'installation,
    ; peut-être pas celle de Windows aujourd'hui : on supprime les quatre.
    Delete "$SMPROGRAMS\Image Print Layout\Uninstall.lnk"
    Delete "$SMPROGRAMS\Image Print Layout\Désinstaller.lnk"
    Delete "$SMPROGRAMS\Image Print Layout\Desinstalar.lnk"
    Delete "$SMPROGRAMS\Image Print Layout\Deinstallieren.lnk"
    RMDir "$SMPROGRAMS\Image Print Layout"
    Delete "$DESKTOP\Image Print Layout.lnk"

    DeleteRegKey HKLM "${UNINSTALL_KEY}"
    DeleteRegKey HKLM "${APP_KEY}"
SectionEnd
