package importer

import (
	"database/sql"
	"encoding/csv"
	"fmt"
	"fyne.io/fyne/v2/widget"
	"github.com/devakdogan/go_csv_adapter/internal/db"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type LoadingHandle struct {
	stopChan      chan bool
	animationDone chan bool
}

func StartLoadingAnimation(logOutput *widget.TextGrid, baseMessage string) *LoadingHandle {
	stopChan := make(chan bool)
	animationDone := make(chan bool)

	go func() {
		loadingStates := []string{".", "..", "..."}
		currentText := logOutput.Text()
		lastLineIndex := strings.LastIndex(currentText, "\n")
		if lastLineIndex == -1 {
			lastLineIndex = 0
		} else {
			lastLineIndex += 1
		}
		baseText := currentText[:lastLineIndex]
		i := 0

		for {
			select {
			case <-stopChan:
				timestamp := time.Now().Format("15:04:05")
				finalText := fmt.Sprintf("%s[%s] %s - Complete\n",
					baseText, timestamp, baseMessage)
				logOutput.SetText(finalText)
				logOutput.Refresh()
				animationDone <- true
				return
			default:
				timestamp := time.Now().Format("15:04:05")
				animationText := fmt.Sprintf("%s[%s] %s%s",
					baseText, timestamp, baseMessage, loadingStates[i])
				logOutput.SetText(animationText)
				logOutput.Refresh()
				i = (i + 1) % len(loadingStates)
				time.Sleep(500 * time.Millisecond)
			}
		}
	}()

	return &LoadingHandle{
		stopChan:      stopChan,
		animationDone: animationDone,
	}
}

func (h *LoadingHandle) Stop() {
	h.stopChan <- true
	<-h.animationDone
}

func appendLog(grid *widget.TextGrid, message string) {
	timestamp := time.Now().Format("15:04:05")
	logLine := fmt.Sprintf("[%s] %s\n", timestamp, message)

	currentText := grid.Text()
	grid.SetText(currentText + logLine)

	grid.Refresh()
}

func createDBProvider(dbType string, config *db.DbConfig) (db.DBProvider, error) {
	switch dbType {
	case "PostgreSQL":
		return &db.Postgres{Config: config.ToPostgresConfig()}, nil
	case "MySQL":
		return &db.MySQL{Config: config.ToMySQLConfig()}, nil
	case "SQLite":
		return &db.SQLite{Config: config.ToSQLiteConfig()}, nil
	default:
		return nil, fmt.Errorf("unsupported database type: %s", dbType)
	}
}

func readCSVHeadersAndSamples(filePath string, sampleLimit int) ([]string, [][]string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, nil, err
	}
	defer func(f *os.File) {
		err := f.Close()
		if err != nil {

		}
	}(f)

	r := csv.NewReader(f)
	r.FieldsPerRecord = -1

	headers, err := r.Read()
	if err != nil {
		return nil, nil, err
	}

	var samples [][]string
	for i := 0; i < sampleLimit; i++ {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}
		samples = append(samples, record)
	}
	return headers, samples, nil
}

func inferColumnTypes(headers []string, samples [][]string) []string {
	types := make([]string, len(headers))

	for _, row := range samples {
		for i, val := range row {
			if types[i] == "string" {
				continue
			}
			if _, err := strconv.Atoi(val); err == nil {
				types[i] = "int"
				continue
			}
			if _, err := strconv.ParseFloat(val, 64); err == nil {
				types[i] = "float"
				continue
			}
			if _, err := time.Parse("2006-01-02", val); err == nil {
				types[i] = "date"
				continue
			}
			types[i] = "string"
		}
	}
	return types
}

func GenerateCreateTableSQL(tableName string, headers []string, types []string) string {
	escapedTable := EscapeIdentifier(tableName)
	stmt := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (", escapedTable)
	for i, col := range headers {
		sqlType := "TEXT"
		switch types[i] {
		case "int":
			sqlType = "INTEGER"
		case "float":
			sqlType = "REAL"
		case "date":
			sqlType = "DATE"
		}
		stmt += fmt.Sprintf("%s %s", EscapeIdentifier(col), sqlType)
		if i < len(headers)-1 {
			stmt += ", "
		}
	}
	stmt += ");"
	return stmt
}
func EscapeIdentifier(s string) string {
	return fmt.Sprintf(`"%s"`, s)
}

func InsertCSVRecords(dbConn *sql.DB, tableName string, headers []string, records [][]string, dbType string) error {
	// Escape the table name to prevent SQL injection
	escapedTable := EscapeIdentifier(tableName)

	// Prepare column names and placeholders arrays
	escapedCols := make([]string, len(headers))
	placeholders := make([]string, len(headers))

	// Process each header column
	for i := range headers {
		// Escape column names
		escapedCols[i] = EscapeIdentifier(headers[i])

		// Use database-specific placeholders
		switch dbType {
		case "PostgreSQL":
			// PostgreSQL uses $1, $2, $3 format for parameters
			placeholders[i] = fmt.Sprintf("$%d", i+1)
		case "MySQL":
			// MySQL can use ? placeholders
			placeholders[i] = "?"
		case "SQLite":
			// SQLite uses ? placeholders
			placeholders[i] = "?"
		default:
			// Default to ? as placeholder for other databases
			placeholders[i] = "?"
		}
	}

	// Construct the SQL insert statement
	insertStmt := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		escapedTable,
		strings.Join(escapedCols, ", "),
		strings.Join(placeholders, ", "),
	)

	// Prepare the statement for execution
	stmt, err := dbConn.Prepare(insertStmt)
	if err != nil {
		return fmt.Errorf("failed to prepare insert statement: %v", err)
	}
	defer stmt.Close()

	// Insert each record from the CSV
	for _, row := range records {
		// Create arguments slice from row values
		args := make([]interface{}, len(row))
		for i, val := range row {
			args[i] = val
		}

		// Execute the prepared statement with row values
		if _, err := stmt.Exec(args...); err != nil {
			return fmt.Errorf("insert failed for row: %v, error: %v", row, err)
		}
	}

	return nil
}

func ImportCSVFiles(folderPath string, dbType string, config *db.DbConfig, logOutput *widget.TextGrid) {
	// Start loading animation in a separate goroutine
	stopAnimation := make(chan bool)
	animationDone := make(chan bool)

	dbConnection := StartLoadingAnimation(logOutput, fmt.Sprintf("Connecting to %s database", dbType))

	// Attempt to create database provider
	provider, err := createDBProvider(dbType, config)

	// Small delay to show animation
	time.Sleep(1 * time.Second)

	// Stop animation and wait for it to complete properly
	dbConnection.Stop()

	// Continue with normal import process
	if err != nil {
		appendLog(logOutput, fmt.Sprintf("Error creating database provider: %v", err))
		return
	}

	// Connecting to database with loading animation
	// Start another loading animation for connection
	go func() {
		loadingStates := []string{".", "..", "..."}
		baseMessage := "Establishing database connection"

		// Save the current log text
		currentText := logOutput.Text()
		lastLineIndex := strings.LastIndex(currentText, "\n")
		if lastLineIndex == -1 {
			lastLineIndex = 0
		} else {
			lastLineIndex += 1
		}
		baseText := currentText[:lastLineIndex]

		i := 0
		for {
			select {
			case <-stopAnimation:
				// Important: Add a new line after animation stops
				timestamp := time.Now().Format("15:04:05")
				finalText := fmt.Sprintf("%s[%s] %s - Complete\n",
					baseText, timestamp, baseMessage)
				logOutput.SetText(finalText)
				logOutput.Refresh()
				animationDone <- true
				return
			default:
				timestamp := time.Now().Format("15:04:05")
				animationText := fmt.Sprintf("%s[%s] %s%s",
					baseText, timestamp, baseMessage, loadingStates[i])

				logOutput.SetText(animationText)
				logOutput.Refresh()

				i = (i + 1) % len(loadingStates)
				time.Sleep(500 * time.Millisecond)
			}
		}
	}()

	// Connect to database
	dbConn, err := provider.Connect()

	// Small delay to show animation
	time.Sleep(2 * time.Second)

	// Stop animation and wait for it to complete properly
	stopAnimation <- true
	<-animationDone

	if err != nil {
		appendLog(logOutput, fmt.Sprintf("DB connection failed: %v", err))
		return
	}

	appendLog(logOutput, "Database connection established successfully!")

	defer func(dbConn *sql.DB) {
		err := dbConn.Close()
		if err != nil {
			appendLog(logOutput, fmt.Sprintf("Error closing database connection: %v", err))
		}
	}(dbConn)

	files, err := os.ReadDir(folderPath)
	if err != nil {
		appendLog(logOutput, fmt.Sprintf("Error reading folder: %v", err))
		return
	}

	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".csv") {
			continue
		}

		filePath := filepath.Join(folderPath, file.Name())
		appendLog(logOutput, fmt.Sprintf("Processing file: %s", file.Name()))

		headers, samples, err := readCSVHeadersAndSamples(filePath, 10)
		if err != nil {
			appendLog(logOutput, fmt.Sprintf("Error reading CSV: %v", err))
			continue
		}

		types := inferColumnTypes(headers, samples)

		rawName := strings.TrimSuffix(file.Name(), ".csv")
		tableName := fmt.Sprintf("%s", rawName)

		// CREATE TABLE
		createSQL := GenerateCreateTableSQL(tableName, headers, types)
		_, err = dbConn.Exec(createSQL)
		if err != nil {
			appendLog(logOutput, fmt.Sprintf("Error creating table %s: %v", tableName, err))
			continue
		}

		f, err := os.Open(filePath)
		if err != nil {
			appendLog(logOutput, fmt.Sprintf("Error reopening CSV: %v", err))
			continue
		}
		defer f.Close()

		r := csv.NewReader(f)
		r.FieldsPerRecord = -1
		_, _ = r.Read()

		var records [][]string
		for {
			record, err := r.Read()
			if err == io.EOF {
				break
			}
			if err != nil {
				appendLog(logOutput, fmt.Sprintf("Error reading row: %v", err))
				break
			}
			records = append(records, record)
		}

		err = InsertCSVRecords(dbConn, tableName, headers, records, dbType)
		if err != nil {
			appendLog(logOutput, fmt.Sprintf("Insert error for %s: %v", tableName, err))
		} else {
			appendLog(logOutput, fmt.Sprintf("Imported into table: %s", tableName))
		}
	}
}
