package main

import (
	"archive/zip"
	"compress/gzip"
	"context"
	"encoding/json"
	"log"
	"os"
	"path"
	"slices"
	"strings"
)

// App struct
type App struct {
	ctx      context.Context
	Registry *Registry
}

type MissingFilesError struct {
	Files []string
}

type MapAlreadyExistsError struct {
	MapCode string
}

func (e *MapAlreadyExistsError) Error() string {
	return "Map with code '" + e.MapCode + "' has already been installed or would overwrite a vanilla map."
}

type FileFoundStruct struct {
	found      bool
	fileObject *zip.File
	required   bool
}

type ConfigData struct {
	name             string
	code             string
	description      string
	population       int
	country          *string
	thumbnailBbox    *[4]float64
	creator          string
	version          string
	initialViewState struct {
		latitude  float64
		longitude float64
		zoom      float64
		pitch     *float64
		bearing   float64
	}
}

func (e *MissingFilesError) Error() string {
	return "Missing required files: " + strings.Join(e.Files, ", ")
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{
		Registry: NewRegistry(),
	}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// Initialize the registry (clone or update) on startup
	if err := a.Registry.Initialize(); err != nil {
		log.Printf("Warning: failed to initialize registry: %v", err)
	}
}

func (a *App) installMap(ctx context.Context, zipFilePath string, subwayBuilderDataPath string) error {
	reader, err := zip.OpenReader(zipFilePath)
	if err != nil {
		return err
	}
	defer reader.Close()

	filesFound := map[string]FileFoundStruct{
		"config":     {found: false, fileObject: nil, required: true},
		"demandData": {found: false, fileObject: nil, required: true},
		"roads":      {found: false, fileObject: nil, required: true},
		"runways":    {found: false, fileObject: nil, required: true},
		"buildings":  {found: false, fileObject: nil, required: true},
		"tiles":      {found: false, fileObject: nil, required: true},
		"oceanDepth": {found: false, fileObject: nil, required: false},
		"thumbnail":  {found: false, fileObject: nil, required: false},
	}

	for _, file := range reader.File {
		if file.FileInfo().IsDir() {
			continue
		}
		fileFound := ""
		switch file.Name {
		case "config.json":
			fileFound = "config"
		case "demand_data.json":
			fileFound = "demandData"
		case "roads.geojson":
			fileFound = "roads"
		case "runways_taxiways.geojson":
			fileFound = "runways"
		case "buildings_index.json":
			fileFound = "buildings"
		case "ocean_depth_index.json":
			fileFound = "oceanDepth"
		}
		if strings.HasSuffix(file.Name, ".pmtiles") {
			fileFound = "tiles"
		}
		if strings.HasSuffix(file.Name, ".svg") {
			fileFound = "thumbnail"
		}
		if fileFound != "" {
			filesFound[fileFound] = FileFoundStruct{found: true, fileObject: file, required: filesFound[fileFound].required}
		}
	}

	missingRequiredFiles := []string{}
	for key, fileInfo := range filesFound {
		if fileInfo.required && !fileInfo.found {
			missingRequiredFiles = append(missingRequiredFiles, key)
		}
	}
	if len(missingRequiredFiles) > 0 {
		return &MissingFilesError{Files: missingRequiredFiles}
	}

	configFile, err := filesFound["config"].fileObject.Open()
	if err != nil {
		return err
	}
	defer configFile.Close()
	fileBytes := make([]byte, filesFound["config"].fileObject.FileInfo().Size())
	_, err = configFile.Read(fileBytes)
	if err != nil {
		return err
	}

	var configData ConfigData
	err = json.Unmarshal(fileBytes, &configData)

	if err != nil {
		return err
	}

	installedMaps := a.Registry.GetInstalledMapCodes()
	vanillaMaps := a.GetVanillaMapCodes()

	if slices.Contains(installedMaps, configData.code) || slices.Contains(vanillaMaps, configData.code) {
		return &MapAlreadyExistsError{MapCode: configData.code}
	}

	os.MkdirAll(path.Join(subwayBuilderDataPath, "cities", "data", configData.code), os.ModePerm)

	for entry, fileInfo := range filesFound {
		if fileInfo.found && entry != "config" && entry != "thumbnail" {
			srcFile, err := fileInfo.fileObject.Open()
			if err != nil {
				return err
			}
			defer srcFile.Close()

			destFilePath := path.Join(subwayBuilderDataPath, "cities", "data", configData.code, path.Base(fileInfo.fileObject.Name)+".gz")
			destFile, err := os.Create(destFilePath)
			if err != nil {
				return err
			}
			defer destFile.Close()
			compressedWriter := gzip.NewWriter(destFile)
			defer compressedWriter.Close()
			fileContent := make([]byte, fileInfo.fileObject.FileInfo().Size())
			_, err = srcFile.Read(fileContent)
			compressedWriter.Write(fileContent)
			if err != nil {
				return err
			}
		}
		if fileInfo.found && entry == "thumbnail" {
			srcFile, err := fileInfo.fileObject.Open()
			if err != nil {
				return err
			}
			defer srcFile.Close()
			srcFileContent := make([]byte, fileInfo.fileObject.FileInfo().Size())
			cityMapsExists, err := os.Stat(path.Join(subwayBuilderDataPath, "public", "data", "city-maps"))
			if os.IsNotExist(err) || !cityMapsExists.IsDir() {
				err = os.MkdirAll(path.Join(subwayBuilderDataPath, "public", "data", "city-maps"), os.ModePerm)
				if err != nil {
					return err
				}
			}
			destFilePath := path.Join(subwayBuilderDataPath, "public", "data", "city-maps", configData.code+".svg")
			destFile, err := os.Create(destFilePath)
			if err != nil {
				return err
			}
			defer destFile.Close()
			_, err = srcFile.Read(srcFileContent)
			if err != nil {
				return err
			}
			_, err = destFile.Write(srcFileContent)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// GetVanillaMapCodes returns the city codes of maps included with the app.
// Currently stubbed to return an empty slice.
func (a *App) GetVanillaMapCodes() []string {
	return []string{}
}
