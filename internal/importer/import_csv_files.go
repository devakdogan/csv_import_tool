package importer

import (
	"database/sql"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2/widget"
	"github.com/devakdogan/go_csv_adapter/internal/db"
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

func BulkInsertCSVRecords(
	dbConn *sql.DB,
	tableName string,
	headers []string,
	records [][]string,
	dbType string,
	batchSize int,
	workerCount int,
	logOutput *widget.TextGrid,
	updateProgress func(int, int),
) error {
	// Ensure we don't create more workers than needed
	totalBatches := (len(records) + batchSize - 1) / batchSize
	if workerCount > totalBatches {
		workerCount = totalBatches
	}
	if workerCount <= 0 {
		workerCount = 1
	}

	var wg sync.WaitGroup
	tasks := make(chan [][]string, workerCount*2) // Buffer channel to avoid blocking
	errChan := make(chan error, workerCount)
	total := len(records)
	progress := make([]int, workerCount)
	progressLock := sync.Mutex{}

	// Initialize progress to 0%
	updateProgress(0, 0)

	// Create workers
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for batch := range tasks {
				// Add a small delay to avoid database overload
				time.Sleep(10 * time.Millisecond)

				if err := insertBatch(dbConn, tableName, headers, batch, dbType); err != nil {
					errChan <- fmt.Errorf("worker %d: %v", workerID, err)
					appendLog(logOutput, fmt.Sprintf("Worker-%02d error: %v", workerID, err))
				} else {
					progressLock.Lock()
					progress[workerID-1] += len(batch)
					// Calculate total progress across all workers
					totalProcessed := 0
					for _, p := range progress {
						totalProcessed += p
					}
					percent := int(float64(totalProcessed) / float64(total) * 100)
					if percent > 100 {
						percent = 100
					}
					// Use 0 as workerID since we're only updating a single progress bar
					updateProgress(0, percent)
					progressLock.Unlock()
				}
			}
		}(i + 1)
	}

	// Distribute tasks to workers
	// Send batches to workers
	for i := 0; i < len(records); i += batchSize {
		endIndex := i + batchSize
		if endIndex > len(records) {
			endIndex = len(records)
		}
		tasks <- records[i:endIndex]
	}

	close(tasks)
	wg.Wait()
	close(errChan)

	// Check for errors
	var errs []error
	for err := range errChan {
		if err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		errMsg := fmt.Sprintf("%d errors occurred during import", len(errs))
		appendLog(logOutput, errMsg)
		return fmt.Errorf("%s", errMsg)
	}

	// Set progress bar to 100% when complete
	updateProgress(0, 100)

	return nil
}
func insertBatch(dbConn *sql.DB, tableName string, headers []string, records [][]string, dbType string) error {
	if len(records) == 0 {
		return nil
	}

	escapedTable := EscapeIdentifier(tableName)
	escapedCols := make([]string, len(headers))
	for i, h := range headers {
		escapedCols[i] = EscapeIdentifier(h)
	}

	var placeholders []string
	var args []interface{}
	argIndex := 1

	for _, record := range records {
		phs := make([]string, len(record))
		for j, val := range record {
			ph := "?"
			if dbType == "PostgreSQL" {
				ph = fmt.Sprintf("$%d", argIndex)
				argIndex++
			}
			phs[j] = ph
			args = append(args, val)
		}
		placeholders = append(placeholders, fmt.Sprintf("(%s)", strings.Join(phs, ", ")))
	}

	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES %s",
		escapedTable,
		strings.Join(escapedCols, ", "),
		strings.Join(placeholders, ", "))

	_, err := dbConn.Exec(query, args...)
	return err
}

func EscapeIdentifier(s string) string {
	return fmt.Sprintf("\"%s\"", s)
}

func ImportCSVFiles(folderPath string, dbType string, config *db.DbConfig, logOutput *widget.TextGrid, updateProgress func(int, int)) { // Start loading animation in a separate goroutine
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

		err = BulkInsertCSVRecords(dbConn, tableName, headers, records, dbType, 1000, 10, logOutput, updateProgress)
		if err != nil {
			appendLog(logOutput, fmt.Sprintf("Insert error for %s: %v", tableName, err))
		} else {
			appendLog(logOutput, fmt.Sprintf("Imported into table: %s", tableName))
		}
	}
}
