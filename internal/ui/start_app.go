package ui

import (
	"fmt"
	"github.com/devakdogan/go_csv_adapter/internal/db"
	"github.com/devakdogan/go_csv_adapter/internal/importer"
	"image/color"
	"strconv"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

var translations = map[string]map[string]string{
	"English": {
		"DatabaseType":     "Database Type:",
		"SelectFolder":     "CSV Folder:",
		"ChooseFolder":     "Choose Folder",
		"Host":             "Host",
		"Port":             "Port",
		"User":             "User",
		"Password":         "Password",
		"Database":         "Database",
		"Language":         "Language:",
		"NoFolderSelected": "Not selected",
		"StartImport":      "Start Import",
		"Confirm":          "Confirm",
		"Edit":             "Edit",
		"Close":            "Close",
		"ConfigureDB":      "Configure Database",
	},
	"Türkçe": {
		"DatabaseType":     "Veritabanı Türü:",
		"SelectFolder":     "CSV Klasörü:",
		"ChooseFolder":     "Klasör Seç",
		"Host":             "Sunucu",
		"Port":             "Port",
		"User":             "Kullanıcı",
		"Password":         "Şifre",
		"Database":         "Veritabanı",
		"Language":         "Dil:",
		"NoFolderSelected": "Seçilmedi",
		"StartImport":      "İçe Aktar",
		"Confirm":          "Tamam",
		"Edit":             "Düzenle",
		"Close":            "Kapat",
		"ConfigureDB":      "Veritabanı Yapılandırması",
	},
}

type dbConfig struct {
	Host       *widget.Entry
	Port       *widget.Entry
	User       *widget.Entry
	Password   *widget.Entry
	Database   *widget.Entry
	Configured bool
}

// Load custom icons
func loadIcon(path string) *canvas.Image {
	img := canvas.NewImageFromFile(path)
	img.FillMode = canvas.ImageFillContain
	img.SetMinSize(fyne.NewSize(48, 48))
	img.Resize(fyne.NewSize(48, 48))
	return img
}

// Variables for auto-scrolling logs
var logScroll *container.Scroll
var logOutput *widget.TextGrid

// Variable for progress bar
var progressBar *widget.ProgressBar

func StartApp() {
	a := app.NewWithID("csv-import-tool")
	w := a.NewWindow("CSV Import Tool")
	w.SetMaster()
	w.Resize(fyne.NewSize(800, 600))
	a.Settings().SetTheme(theme.DarkTheme())

	currentLang := "English"
	selectedDB := new(string)
	config := &dbConfig{}
	isPopupOpen := new(bool)
	folderPath := widget.NewLabel(translations[currentLang]["NoFolderSelected"])
	folderPath.Wrapping = fyne.TextTruncate

	// Initialize the log output and scroll container
	logOutput = widget.NewTextGrid()
	logScroll = container.NewScroll(logOutput)
	logScroll.SetMinSize(fyne.NewSize(700, 200))
	
	// Create a single progress bar for the current CSV file
	progressBar = widget.NewProgressBar()
	progressBar.Min = 0
	progressBar.Max = 100
	progressBar.SetValue(0)
	// Make percentage text centered and white
	progressBar.TextFormatter = func() string {
		return fmt.Sprintf("%d%%", int(progressBar.Value))
	}
	
	updateProgress := func(workerID int, percent int) {
		// Update the progress bar directly
		progressBar.SetValue(float64(percent))
	}

	var updateUI func()
	updateUI = func() {
		w.SetContent(buildUI(w, &currentLang, config, selectedDB, folderPath, updateUI, isPopupOpen, logOutput, updateProgress))
	}
	updateUI()
	w.ShowAndRun()
}

// Özel tema yapısı
type customTheme struct {
	fyne.Theme
}

func (t *customTheme) Color(n fyne.ThemeColorName, v fyne.ThemeVariant) color.Color {
	switch n {
	case theme.ColorNameBackground:
		return color.Black
	case theme.ColorNameForeground:
		return color.White
	case theme.ColorNameButton:
		return color.Black
	case theme.ColorNameDisabled:
		return color.Gray{Y: 0x42}
	case theme.ColorNamePlaceHolder:
		return color.Gray{Y: 0x88}
	case theme.ColorNameScrollBar:
		return color.Gray{Y: 0x88}
	default:
		return theme.DefaultTheme().Color(n, v)
	}
}

func buildUI(w fyne.Window, lang *string, config *dbConfig, selectedDB *string, folderPath *widget.Label,
	refreshFunc func(), isPopupOpen *bool, logOutput *widget.TextGrid, 
	updateProgress func(int, int)) fyne.CanvasObject {
	t := translations[*lang]

	langSelect := widget.NewSelect([]string{"English", "Türkçe"}, func(selected string) {
		*lang = selected
		refreshFunc()
	})
	langSelect.Selected = *lang
	langSelect.Resize(fyne.NewSize(120, 35))
	topRight := container.NewHBox(layout.NewSpacer(), langSelect)

	dbTypes := []string{"PostgreSQL", "MySQL", "SQLite"}
	var dbCards []fyne.CanvasObject
	dbTitle := widget.NewLabelWithStyle("Database", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

	for _, db := range dbTypes {
		dbNameCopy := db
		var icon *canvas.Image
		switch db {
		case "PostgreSQL":
			icon = loadIcon("assets/icons/postgre.png")
		case "MySQL":
			icon = loadIcon("assets/icons/mysql.png")
		case "SQLite":
			icon = loadIcon("assets/icons/sqlite.png")
		}
		icon.SetMinSize(fyne.NewSize(48, 48))
		icon.Resize(fyne.NewSize(48, 48))

		dbButton := widget.NewButton("", func() {
			if *isPopupOpen {
				return
			}
			if *selectedDB != dbNameCopy {
				*config = dbConfig{}
				*selectedDB = dbNameCopy
				*isPopupOpen = true
				showDBPopup(w, lang, config, dbNameCopy, func() {
					refreshFunc()
				}, func() {
					*isPopupOpen = false
				})
			}
		})

		isSelected := *selectedDB == dbNameCopy
		if isSelected {
			dbButton.Importance = widget.HighImportance
			editButton := widget.NewButton(t["Edit"], func() {
				if *isPopupOpen {
					return
				}
				*isPopupOpen = true
				showDBPopup(w, lang, config, dbNameCopy, func() {
					refreshFunc()
				}, func() {
					*isPopupOpen = false
				})
			})
			editButton.Resize(fyne.NewSize(120, 30))
			cardContainer := container.NewVBox(
				container.NewMax(
					dbButton,
					container.NewVBox(
						container.NewCenter(icon),
						container.NewCenter(widget.NewLabel(dbNameCopy)),
					),
				),
				editButton,
			)
			dbCards = append(dbCards, cardContainer)
		} else {
			cardContainer := container.NewMax(
				dbButton,
				container.NewVBox(
					container.NewCenter(icon),
					container.NewCenter(widget.NewLabel(dbNameCopy)),
				),
			)
			dbCards = append(dbCards, cardContainer)
		}
	}
	dbContainer := container.NewGridWithColumns(3)
	for _, card := range dbCards {
		cardContainer := container.NewMax(card)
		cardContainer.Resize(fyne.NewSize(220, 150))
		dbContainer.Add(cardContainer)
	}
	dbContainer.Add(layout.NewSpacer())
	dbBorder := canvas.NewRectangle(theme.ForegroundColor())
	dbBorder.StrokeColor = theme.ForegroundColor()
	dbBorder.StrokeWidth = 1
	dbBorder.FillColor = theme.BackgroundColor()
	dbSection := container.NewVBox(dbTitle, container.NewPadded(dbContainer))
	dbBox := container.NewMax(dbBorder, container.NewPadded(dbSection))

	logsTitle := widget.NewLabelWithStyle("Logs", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	logsBorder := canvas.NewRectangle(theme.ForegroundColor())
	logsBorder.StrokeColor = theme.ForegroundColor()
	logsBorder.StrokeWidth = 1
	logsBorder.FillColor = theme.BackgroundColor()
	logsSection := container.NewBorder(logsTitle, nil, nil, nil, container.NewPadded(logScroll))
	logsBox := container.NewMax(logsBorder, container.NewPadded(logsSection))

	// Create a container for the progress bar
	progressLabel := widget.NewLabel("Import Progress:")
	progressLabel.TextStyle = fyne.TextStyle{Bold: true}
	
	progressContainer := container.NewVBox(
		progressLabel,
		progressBar,
	)
	progressContainer.Resize(fyne.NewSize(700, 60))
	
	progressBox := container.NewPadded(progressContainer)

	pathLabel := widget.NewLabel("CSV Path: ")
	pathLabel.TextStyle.Bold = true
	pathText := widget.NewLabel(folderPath.Text)
	pathContainer := container.NewHBox(pathLabel, pathText)

	folderButton := widget.NewButton(t["ChooseFolder"], func() {
		if *isPopupOpen {
			return
		}
		dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
			if uri != nil {
				folderPath.SetText(uri.Path())
				refreshFunc()
			}
		}, w)
	})
	folderButton.Resize(fyne.NewSize(150, 40))

	importButton := widget.NewButton(t["StartImport"], func() {
		if *isPopupOpen {
			return
		}
		if *selectedDB == "" {
			appendLog(logOutput, "Error: Please select a database type first")
			return
		}
		if !config.Configured {
			appendLog(logOutput, "Error: Please configure the database connection first")
			return
		}
		if folderPath.Text == t["NoFolderSelected"] {
			appendLog(logOutput, "Error: Please select a CSV folder first")
			return
		}

		appendLog(logOutput, fmt.Sprintf("Starting import process for %s database...", *selectedDB))
		appendLog(logOutput, fmt.Sprintf("Using folder: %s", folderPath.Text))
		appendLog(logOutput, fmt.Sprintf("Database: %s@%s:%s/%s", config.User.Text, config.Host.Text, config.Port.Text, config.Database.Text))

		importer.ImportCSVFiles(folderPath.Text, *selectedDB, (*db.DbConfig)(config), logOutput, func(workerID int, percent int) {
			// Update the progress bar directly
			progressBar.SetValue(float64(percent))
		})
	})
	importButton.Resize(fyne.NewSize(150, 40))

	bottomSection := container.NewHBox(folderButton, layout.NewSpacer(), importButton)
	mainContent := container.NewVBox(
		topRight,
		container.NewPadded(dbBox),
		container.NewPadded(logsBox),
		progressBox,
		pathContainer,
		bottomSection,
	)
	return container.NewPadded(mainContent)
}

func showDBPopup(mainWindow fyne.Window, lang *string, config *dbConfig, dbType string, onConfirm func(), onClose func()) {
	t := translations[*lang]

	if config.Host == nil {
		config.Host = widget.NewEntry()
	}
	if config.Port == nil {
		config.Port = widget.NewEntry()
	}
	config.Port.OnChanged = func(s string) {
		filtered := ""
		for _, r := range s {
			if r >= '0' && r <= '9' {
				filtered += string(r)
			}
		}
		if len(filtered) > 5 {
			filtered = filtered[:5]
		}
		if filtered != s {
			config.Port.SetText(filtered)
			return
		}
		if filtered != "" {
			if val, err := strconv.Atoi(filtered); err == nil && val > 65536 {
				config.Port.SetText("65536")
			}
		}
	}
	if config.User == nil {
		config.User = widget.NewEntry()
	}
	if config.Password == nil {
		config.Password = widget.NewPasswordEntry()
	}
	if config.Database == nil {
		config.Database = widget.NewEntry()
	}

	if config.Configured {
		config.Host.SetText(config.Host.Text)
		config.Port.SetText(config.Port.Text)
		config.User.SetText(config.User.Text)
		config.Password.SetText(config.Password.Text)
		config.Database.SetText(config.Database.Text)
	}

	if !config.Configured {
		switch dbType {
		case "PostgreSQL":
			config.Host.SetText("localhost")
			config.Port.SetText("5432")
			config.User.SetText("postgres")
			config.Database.SetText("postgres")
		case "MySQL":
			config.Host.SetText("localhost")
			config.Port.SetText("3306")
			config.User.SetText("root")
			config.Database.SetText("mysql")
		case "SQLite":
			config.Host.SetText("localhost")
			config.Port.SetText("0")
			config.User.SetText("root")
			config.Database.SetText("local.db")
		}
	}

	form := widget.NewForm(
		&widget.FormItem{Text: t["Host"], Widget: config.Host},
		&widget.FormItem{Text: t["Port"], Widget: config.Port},
		&widget.FormItem{Text: t["User"], Widget: config.User},
		&widget.FormItem{Text: t["Password"], Widget: config.Password},
		&widget.FormItem{Text: t["Database"], Widget: config.Database},
	)

	// Butonlar için container
	var customDialog dialog.Dialog

	closeBtn := widget.NewButton(t["Close"], func() {
		if customDialog != nil {
			customDialog.Hide()
		}
		onClose()
	})

	confirmBtn := widget.NewButton(t["Confirm"], func() {
		if config.Host.Text == "" || config.Port.Text == "" || config.User.Text == "" || config.Password.Text == "" || config.Database.Text == "" {
			dialog.ShowError(fmt.Errorf("All fields must be filled"), mainWindow)
			return
		}

		config.Configured = true
		onConfirm()
		customDialog.Hide()
	})

	buttonBox := container.NewHBox(
		layout.NewSpacer(),
		confirmBtn,
		closeBtn,
		layout.NewSpacer(),
	)

	// Border ile çerçeve oluştur
	formBorder := canvas.NewRectangle(theme.BackgroundColor())
	formBorder.StrokeColor = theme.ForegroundColor()
	formBorder.StrokeWidth = 1

	formContainer := container.NewMax(
		formBorder,
		container.NewPadded(form),
	)

	content := container.NewVBox(
		widget.NewLabelWithStyle(t["ConfigureDB"], fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		formContainer,
		buttonBox,
	)

	// Create a custom dialog
	customDialog = dialog.NewCustomWithoutButtons(t["ConfigureDB"], content, mainWindow)
	customDialog.Resize(fyne.NewSize(500, 400))
	customDialog.SetOnClosed(onClose)
	customDialog.Show()
}

// Log mesajlarını eklemek için yardımcı fonksiyon
// Using the package-level globalLogScroll variable

func appendLog(grid *widget.TextGrid, message string) {
	timestamp := time.Now().Format("15:04:05")
	logLine := fmt.Sprintf("[%s] %s\n", timestamp, message)

	currentText := grid.Text()
	grid.SetText(currentText + logLine)

	// Refresh the grid
	grid.Refresh()
	
	// Auto-scroll to the bottom
	if logScroll != nil {
		// Use a goroutine to ensure the scroll happens after the UI updates
		go func() {
			time.Sleep(50 * time.Millisecond)
			logScroll.ScrollToBottom()
		}()
	}
}
