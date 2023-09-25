
**Prompt**: Is there any wierd behviour or bugs in this code Format the response as markdown without enclosing backticks.

Is there any wierd behviour or bugs in this code Format the response as markdown without enclosing backticks.

package main

import (
	"errors"
	"fmt"
	"image"
	"strconv"

	"net/http"

	"github.com/johan-st/go-image-server/units/size"
	"github.com/johan-st/go-image-server/way"
)

// IMAGES

func (srv *server) handleApiImageGet() http.HandlerFunc {
	// setup
	l := srv.errorLogger.With("handler", "handleApiImageGet")

	type resp struct {
		Message      string `json:"message"`
		AvailableIds []int  `json:"availableIds,omitempty"`
	}

	// handler
	return func(w http.ResponseWriter, r *http.Request) {
		ids, err := srv.ih.Ids()
		if err != nil {
			l.Error(err)
			srv.respondJson(w, r, http.StatusInternalServerError, resp{
				Message: "Internal Server Error",
			})
		}
		resp := resp{
			Message:      "listing all available image ids",
			AvailableIds: ids,
		}
		l.Debug(resp)
		srv.respondJson(w, r, http.StatusOK, resp)
	}
}

func (srv *server) handleApiImageDelete() http.HandlerFunc {
	// setup
	l := srv.errorLogger.With("handler", "handleApiImageDelete")

	type badReqResp struct {
		Error string `json:"error"`
		Got   string `json:"got"`
		Want  string `json:"want"`
	}

	// handler
	return func(w http.ResponseWriter, r *http.Request) {
		id_str := way.Param(r.Context(), "id")
		l.Debug("handling delete image request", "id", id_str)

		id, err := strconv.Atoi(id_str)
		if err != nil {
			l.Error("error while parsing id", "id", id_str, "ParseIntError", err)
			srv.respondJson(w, r, http.StatusBadRequest, badReqResp{
				Error: err.Error(),
				Got:   id_str,
				Want:  "int > 0",
			})
			return
		}

		err = srv.ih.Delete(id)
		if err != nil {
			l.Error("error while deleting image", "id", id, "ImageHandlerError", err)
			srv.respondCode(w, r, http.StatusInternalServerError)
			return
		}
		srv.respondCode(w, r, http.StatusOK)
	}
}

// TODO: handle errors and respond with correct status codes
func (srv *server) handleApiImagePost() http.HandlerFunc {
	// setup
	l := srv.errorLogger.With("handler", "handleUpload")
	l.With("version", "1")
	l.With("method", "POST")

	type responseOK struct {
		Status  int    `json:"status"`
		Message string `json:"message"`
		Id      int    `json:"id"`
		Url     string `json:"url"`
	}

	type responseErr struct {
		Status int    `json:"status"`
		Error  string `json:"error"`
	}
	// handler
	// TODO: figure out which erorrs are client errors and which are server errors (warn/info vs error)
	return func(w http.ResponseWriter, r *http.Request) {
		err := r.ParseMultipartForm(int64(15 * size.Megabyte))
		if err != nil {
			l.Warn("Error while parsing upload", "ParseMultipartFormError", err)
			srv.respondJson(w, r, http.StatusBadRequest, responseErr{
				Status: http.StatusBadRequest,
				Error:  err.Error(),
			})
			return
		}

		// Get header for filename, size and headers
		// TODO: can I get the first file from the form without knowing the name?
		upload, header, err := r.FormFile("image")
		if err != nil {
			l.Warn("Error Retrieving the File", "FormFileError", err)
			return
		}
		defer upload.Close()

		// check size
		headerSize := size.S(header.Size)
		maxUploadSize, err := size.Parse(srv.conf.MaxUploadSize)
		if err != nil {
			l.Fatal("Error while parsing max upload size", "ParseSizeError", err)
			srv.respondJson(w, r, http.StatusInternalServerError, responseErr{
				Status: http.StatusInternalServerError,
				Error:  "Internal Server Error",
			})
			return
		}
		if headerSize > maxUploadSize {
			l.Warn("Maximum upload size exceeded", "FileSize", headerSize, "MaxUploadSize", maxUploadSize)
			srv.respondJson(w, r, http.StatusBadRequest, responseErr{Status: http.StatusBadRequest, Error: "Maximum upload size exceeded"})
			return
		}

		// add to image handler
		id, err := srv.ih.Add(upload)
		if err != nil {
			if errors.Is(err, image.ErrFormat) {
				l.Warn("Error while adding image to handler", "AddIOError", err)
				srv.respondJson(w, r, http.StatusBadRequest, responseErr{
					Status: http.StatusBadRequest,
					Error:  "File is not a valid image",
				})
				return
			}
			l.Error("Error while adding image to handler", "AddIOError", err)
		}

		l.Info("File Uploaded Successfully", "assigned id", id, "original filename", header.Filename, "upload size", header.Size)

		// var url string
		// if srv.conf.Port == 0 || srv.conf.Port == 80 {
		// 	url = fmt.Sprintf("http://%s/%d", srv.conf.Host, id)
		// } else {
		// 	url = fmt.Sprintf("http://%s:%d/%d", srv.conf.Host, srv.conf.Port, id)
		// }

		response := responseOK{
			Status:  http.StatusCreated,
			Message: "File Uploaded Successfully",
			Id:      id,
			Url:     fmt.Sprintf("/%d", id),
		}

		srv.respondJson(w, r, http.StatusCreated, response)
	}
}
package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/johan-st/go-image-server/images"
	"github.com/johan-st/go-image-server/units/size"
	"gopkg.in/yaml.v3"
)

type config struct {
	LogLevel      string            `yaml:"log_level"`
	Http          confHttp          `yaml:"http"`
	Files         confFiles         `yaml:"files"`
	Cache         confCache         `yaml:"cache_rules"`
	ImageDefaults confImageDefault  `yaml:"image_defaults"`
	ImagePresets  []confImagePreset `yaml:"image_presets"`
}

type confHttp struct {
	Port          int    `yaml:"port"`
	Host          string `yaml:"host"`
	Docs          bool   `yaml:"documentation"`
	AccessLog     string `yaml:"access_logfile"`
	MaxUploadSize string `yaml:"max_upload_size"`
}

type confFiles struct {
	ClearOnStart bool `yaml:"clear_on_start"`
	ClearOnExit  bool `yaml:"clear_on_exit"`
	SetPerms     bool `yaml:"set_perms"`
	CreateDirs   bool `yaml:"create_dirs"`

	DirOriginals string `yaml:"originals_dir"`
	DirCache     string `yaml:"cache_dir"`
	PopulateFrom string `yaml:"populate_from"`
}

type confCache struct {
	Cap     int    `yaml:"max_objects"`
	MaxSize string `yaml:"max_size"`
}

type confImageDefault struct {
	Format        string `yaml:"format"`
	QualityJpeg   int    `yaml:"quality_jpeg"`
	QualityGif    int    `yaml:"quality_gif"`
	Width         int    `yaml:"width"`
	Height        int    `yaml:"height"`
	MaxSize       string `yaml:"max_size"`
	Interpolation string `yaml:"interpolation"`
}

type confImagePreset struct {
	Name          string   `yaml:"name"`
	Alias         []string `yaml:"alias"`
	Format        string   `yaml:"format,omitempty"`
	Quality       int      `yaml:"quality,omitempty"`
	Width         int      `yaml:"width"`
	Height        int      `yaml:"height"`
	MaxSize       string   `yaml:"max_size,omitempty"`
	Interpolation string   `yaml:"interpolation,omitempty"`
}

func saveConfig(c config, filename string) error {
	bytes, err := yaml.Marshal(c)
	if err != nil {
		return err
	}

	return os.WriteFile(filename, bytes, 0644)
}

func loadConfig(filename string) (config, error) {
	bytes, err := os.ReadFile(filename)
	if err != nil {
		return config{}, err
	}

	var c config
	err = yaml.Unmarshal(bytes, &c)
	if err != nil {
		return config{}, err
	}

	return c, nil
}

// validate enforces config rules and returns an error if any are broken
func (c *config) validate() error {
	errs := []error{}

	// HTTP
	// port needed
	if c.Http.Port == 0 {
		errs = append(errs, fmt.Errorf("server port must be set"))
	}
	if c.Http.MaxUploadSize == "" {
		c.Http.MaxUploadSize = "20MB"
	}
	_, err := size.Parse(c.Http.MaxUploadSize)
	if err != nil {
		errs = append(errs, fmt.Errorf("server max upload size must be a valid size (e.g. 20MB)"))
	}

	// empty host is ok
	// TODO: validate host format

	// FILES
	if c.Files.DirOriginals == "" {
		errs = append(errs, fmt.Errorf("path for originals must be set"))
	}
	if c.Files.DirCache == "" {
		errs = append(errs, fmt.Errorf("paths for cache must be set"))
	}
	if c.Cache.Cap == 0 {
		errs = append(errs, fmt.Errorf("cache num must be greater than 0"))
	}

	// DEFAULT IMAGE PARAMETERS
	if c.ImageDefaults.Format != "jpeg" && c.ImageDefaults.Format != "png" && c.ImageDefaults.Format != "gif" {
		errs = append(errs, fmt.Errorf("default image parameters format must be set to a valid value. Valid values are: jpeg, png, gif"))
	}
	if c.ImageDefaults.QualityJpeg == 0 {
		errs = append(errs, fmt.Errorf("default image parameters quality jpeg must be set to a value greater between 1 and 100 (inclusive)"))
	}
	if c.ImageDefaults.QualityGif == 0 {
		errs = append(errs, fmt.Errorf("default image parameters quality gif must be set to a value greater between 1 and 256 (inclusive)"))
	}
	if c.ImageDefaults.Width == 0 && c.ImageDefaults.Height == 0 {
		errs = append(errs, fmt.Errorf("default image parameters width or height (or both) must be set"))
	}
	// TODO: validate max_size format
	// 0 is ok for max_size, it means no limit
	// TODO: validate resize format

	// IMAGE PARAMETERS
	for _, p := range c.ImagePresets {
		name := p.Name
		if name == "" {
			errs = append(errs, fmt.Errorf("image parameters name must be set"))
		}
		if p.Format != "" && p.Format != "jpeg" && p.Format != "png" && p.Format != "gif" {
			errs = append(errs, fmt.Errorf("image parameters (name: \"%s\") format must be set to a valid value. Valid values are: jpeg, png, gif", name))
		}
		if p.Quality == 0 && p.Format == "jpeg" {
			errs = append(errs, fmt.Errorf("image parameters (name: \"%s\") quality must be set to a value greater between 1 and 100 (inclusive)", name))
		}
		if p.Quality == 0 && p.Format == "gif" {
			errs = append(errs, fmt.Errorf("image parameters (name: \"%s\") quality must be set to a value greater between 1 and 256 (inclusive)", name))
		}
		if p.Width == 0 && p.Height == 0 {
			errs = append(errs, fmt.Errorf("image parameters (name: \"%s\") width or height (or both) must be set", name))
		}

		// TODO: validate max_size format
		// 0 is ok for max_size, it means no limit
		// TODO: validate resize format
	}

	// Return errors if any
	if len(errs) > 0 {
		errs = append(errs, fmt.Errorf("config validation failed"))
		return errors.Join(errs...)
		// return fmt.Errorf("config validation failed: %v", errs)
	}
	return nil
}

// TODO: handle errors by returning them?
func toImageDefaults(c confImageDefault) (images.ImageDefaults, error) {
	errs := []error{}

	format, err := images.ParseFormat(c.Format)
	if err != nil {
		errs = append(errs, err)
	}
	size, err := size.Parse(c.MaxSize)
	if err != nil {
		errs = append(errs, err)
	}

	interpolation, err := images.ParseInterpolation(c.Interpolation)
	if err != nil {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		newErrs := []error{fmt.Errorf("(%d) errors while building ImageDefaults", len(errs))}
		newErrs = append(newErrs, errs...)
		return images.ImageDefaults{}, errors.Join(newErrs...)
	}

	return images.ImageDefaults{
		Format:        format,
		QualityJpeg:   c.QualityJpeg,
		QualityGif:    c.QualityGif,
		Width:         c.Width,
		Height:        c.Height,
		MaxSize:       size,
		Interpolation: interpolation,
	}, nil
}

func toImagePresets(conf []confImagePreset, def images.ImageDefaults) ([]images.ImagePreset, error) {
	presets := []images.ImagePreset{}
	var err error
	errs := []error{}

	for _, cip := range conf {
		// format
		var format images.Format
		if cip.Format != "" {
			format, err = images.ParseFormat(cip.Format)
			if err != nil {
				errs = append(errs, err)
			}
		} else {
			format = def.Format
		}

		// size
		var s size.S
		if cip.MaxSize != "" {
			s, err = size.Parse(cip.MaxSize)
			if err != nil {
				errs = append(errs, err)
			}
		} else {
			s = def.MaxSize
		}

		// interpolation
		var interpolation images.Interpolation
		if cip.Interpolation != "" {
			interpolation, err = images.ParseInterpolation(cip.Interpolation)
			if err != nil {
				errs = append(errs, err)
			}
		} else {
			interpolation = def.Interpolation
		}

		// resulting preset
		p := images.ImagePreset{
			Name:          cip.Name,
			Alias:         cip.Alias,
			Format:        format,
			Quality:       cip.Quality,
			Width:         cip.Width,
			Height:        cip.Height,
			MaxSize:       s,
			Interpolation: interpolation,
		}
		presets = append(presets, p)
	}

	if len(errs) > 0 {
		newErrs := []error{fmt.Errorf("(%d) errors while building ImagePresets", len(errs))}
		newErrs = append(newErrs, errs...)
		return []images.ImagePreset{}, errors.Join(newErrs...)
	}
	return presets, nil
}

func defaultConfig() config {
	return config{
		LogLevel: "info",
		Http: confHttp{
			Port: 8080,
			Host: "",
			Docs: false,
		},
		Files: confFiles{
			ClearOnStart: false,
			PopulateFrom: "",
			SetPerms:     false,
			CreateDirs:   false,
			DirOriginals: "img/originals",
			DirCache:     "img/cached",
		},
		Cache: confCache{
			Cap:     100000,
			MaxSize: "500 GB",
		},
		ImageDefaults: confImageDefault{
			Format:        "jpeg",
			QualityJpeg:   80,
			QualityGif:    256,
			Width:         0,
			Height:        800,
			MaxSize:       "1 MB",
			Interpolation: "nearestNeighbor",
		},
		ImagePresets: []confImagePreset{
			{
				Name:          "thumbnail",
				Alias:         []string{"thumb", "th"},
				Format:        "jpeg",
				Quality:       80,
				Width:         150,
				Height:        150,
				MaxSize:       "10 KB",
				Interpolation: "lanczos3",
			},
			{
				Name:   "small",
				Alias:  []string{"small", "s"},
				Height: 400,
				Width:  0,
			},
			{
				Name:   "medium",
				Alias:  []string{"medium", "m"},
				Height: 800,
				Width:  0,
			},
			{
				Name:   "large",
				Alias:  []string{"large", "l"},
				Height: 1600,
				Width:  0,
			},
		},
	}
}
// Code generated by templ@v0.2.334 DO NOT EDIT.

package main

//lint:file-ignore SA4006 This context is only used if a nested component is present.

import "github.com/a-h/templ"
import "context"
import "io"
import "bytes"

func hello(name string) templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) (err error) {
		templBuffer, templIsBuffer := w.(*bytes.Buffer)
		if !templIsBuffer {
			templBuffer = templ.GetBuffer()
			defer templ.ReleaseBuffer(templBuffer)
		}
		ctx = templ.InitializeContext(ctx)
		var_1 := templ.GetChildren(ctx)
		if var_1 == nil {
			var_1 = templ.NopComponent
		}
		ctx = templ.ClearChildren(ctx)
		_, err = templBuffer.WriteString("<div>")
		if err != nil {
			return err
		}
		var_2 := `Hello, `
		_, err = templBuffer.WriteString(var_2)
		if err != nil {
			return err
		}
		var var_3 string = name
		_, err = templBuffer.WriteString(templ.EscapeString(var_3))
		if err != nil {
			return err
		}
		_, err = templBuffer.WriteString("</div>")
		if err != nil {
			return err
		}
		if !templIsBuffer {
			_, err = templBuffer.WriteTo(w)
		}
		return err
	})
}
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/johan-st/go-image-server/images"
	"github.com/johan-st/go-image-server/units/size"
	"github.com/johan-st/go-image-server/way"
)

func main() {
	l := newCustomLogger()
	log.SetDefault(l)

	err := run()
	if err != nil {
		if errors.Is(err, http.ErrServerClosed) {
			l.Info("server closed")
			os.Exit(0)
			return
		}
		l.Fatal(err)
	}
}

func run() error {
	// parse flags
	flagConf := flag.String("c", "imageServer_config.yaml", "path to configuration file")
	flagDev := flag.Bool("dev", false, "enable source code debugging")
	flagDebug := flag.Bool("debug", false, "enable debug logging regardless of configurations")
	flag.Parse()

	// load configuration
	conf, err := loadConfig(*flagConf)

	// set up logger
	l := log.Default()
	// l.Info("starting server", "version", version, "commit", commit, "build time", buildTime)

	// enable development mode before handling first error. If flag is set
	// i.e. report caller and set log level to debug
	if *flagDev {
		l.SetReportCaller(true)
	}

	// enable debug logging if flag is set
	if *flagDebug {
		l.SetLevel(log.DebugLevel)
		conf.LogLevel = "debug"
	}

	l.Info("starting server...")

	// handle configuration errors
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			l.Error("configuration file not found. creating example configuration in its place", "path", *flagConf)
			saveErr := saveConfig(defaultConfig(), *flagConf)
			if saveErr != nil {
				l.Error("could not save config", "error", saveErr)
				return fmt.Errorf("config file not found. could not create example config file: %w", saveErr)
			}
			return fmt.Errorf("config was not found. created example config in %s", *flagConf)
		}
		return err
	}

	l.Debug("configuration loaded", "file path", *flagConf)
	err = conf.validate()
	if err != nil {
		return err
	}
	l.Debug("configuration loaded and validated", "file path", *flagConf)

	l.SetLevel(log.ParseLevel(conf.LogLevel))

	if conf.Files.ClearOnStart {
		l.Warn(
			"Clearing folders",
			"originals_dir", conf.Files.DirOriginals,
			"cache_dir", conf.Files.DirCache,
		)
		os.RemoveAll(conf.Files.DirOriginals)
		os.RemoveAll(conf.Files.DirCache)
	}

	imageDefaults, err := toImageDefaults(conf.ImageDefaults)
	if err != nil {
		return err
	}

	imagePresets, err := toImagePresets(conf.ImagePresets, imageDefaults)
	if err != nil {
		return err
	}

	cacheMaxSize, err := size.Parse(conf.Cache.MaxSize)
	if err != nil {
		return err
	}

	// create image handler
	ih, err := images.New(
		images.WithLogger(l.WithPrefix("[images]")),
		images.WithLogLevel(conf.LogLevel),

		images.WithCreateDirs(conf.Files.CreateDirs),
		images.WithSetPermissions(conf.Files.SetPerms),

		images.WithOriginalsDir(conf.Files.DirOriginals),
		images.WithCacheDir(conf.Files.DirCache),

		images.WithCacheMaxNum(conf.Cache.Cap),
		images.WithCacheMaxSize(cacheMaxSize),

		images.WithImageDefaults(imageDefaults),
		images.WithImagePresets(imagePresets),
	)
	if err != nil {
		return err
	}

	if conf.Files.PopulateFrom != "" {
		err = addFolder(ih, conf.Files.PopulateFrom)
		if err != nil {
			l.Error("could not populate originals", "error", err)
		}
	}

	if conf.Http.Port == 0 {
		conf.Http.Port = 8000
		l.Info("Port not set in config. Using default port", "port", conf.Http.Port)
	}

	// set up http log

	var al *log.Logger
	if conf.Http.AccessLog != "" {
		l.Info("access log enabled", "path", conf.Http.AccessLog)
		file, err := os.OpenFile(conf.Http.AccessLog, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			l.Error("could not open access log file", "error", err)
		}
		defer file.Close()

		al = log.New(file)
		if path.Ext(conf.Http.AccessLog) == ".json" {
			al.SetFormatter(log.JSONFormatter)
			l.Info("access log format set to json")
		} else {
			al.SetFormatter(log.TextFormatter)
			l.Info("access log format set to text")
		}
	}

	// set up srv
	srv := &server{
		conf:         conf.Http,
		router:       *way.NewRouter(),
		ih:           ih,
		accessLogger: al,
		errorLogger:  l.WithPrefix("[http]"),
	}
	srv.routes()

	// set up server
	mainSrv := &http.Server{
		Addr:              fmt.Sprintf("%s:%d", conf.Http.Host, conf.Http.Port),
		Handler:           srv,
		ReadTimeout:       1 * time.Second,
		ReadHeaderTimeout: 1 * time.Second,
		WriteTimeout:      1 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// graceful shutdown
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)
	go func() {
		for sig := range signalChan {
			l.Info("signal recieved", "signal", sig)
			if conf.Files.ClearOnExit {
				l.Warn(
					"ClearOnExit is set. Removing folders",
					"originals_dir", conf.Files.DirOriginals,
					"cache_dir", conf.Files.DirCache,
				)
				os.RemoveAll(conf.Files.DirOriginals)
				os.RemoveAll(conf.Files.DirCache)
			}
			mainSrv.Shutdown(context.Background())
		}
	}()

	l.Info("server is up and listening", "addr", mainSrv.Addr)
	return mainSrv.ListenAndServe()
}

// LOGGER STUFF

// set up logger
func newCustomLogger() *log.Logger {
	opt := log.Options{
		Prefix:          "[main]",
		Level:           log.InfoLevel,
		ReportCaller:    false,
		CallerFormatter: funcCallerFormater,
		ReportTimestamp: true,
		TimeFormat:      "",
		Formatter:       log.TextFormatter,
		Fields:          []interface{}{},
	}
	return log.NewWithOptions(os.Stderr, opt)
}

func funcCallerFormater(file string, line int, funcName string) string {
	return fmt.Sprintf("%s:%d %s", trimCaller(file, 2, '/'), line, trimCaller(funcName, 2, '.'))
}

// Cleanup a path by returning the last n segments of the path only.
func trimCaller(path string, n int, sep byte) string {
	// lovely borrowed from zap
	// nb. To make sure we trim the path correctly on Windows too, we
	// counter-intuitively need to use '/' and *not* os.PathSeparator here,
	// because the path given originates from Go stdlib, specifically
	// runtime.Caller() which (as of Mar/17) returns forward slashes even on
	// Windows.
	//
	// See https://github.com/golang/go/issues/3335
	// and https://github.com/golang/go/issues/18151
	//
	// for discussion on the issue on Go side.

	// Return the full path if n is 0.
	if n <= 0 {
		return path
	}

	// Find the last separator.
	idx := strings.LastIndexByte(path, sep)
	if idx == -1 {
		return path
	}

	for i := 0; i < n-1; i++ {
		// Find the penultimate separator.
		idx = strings.LastIndexByte(path[:idx], sep)
		if idx == -1 {
			return path
		}
	}

	return path[idx+1:]
}

// Helper function to add all images in a folder to the image handler
func addFolder(ih *images.ImageHandler, folder string) error {
	dir, err := os.Open(folder)
	if err != nil {
		return err
	}
	defer dir.Close()

	files, err := dir.Readdir(0)
	if err != nil {
		return err
	}

	for _, info := range files {
		if info.IsDir() {
			continue
		}
		file, err := os.Open(folder + "/" + info.Name())
		if err != nil {
			log.Default().Info("failed to open image", "file", info.Name(), "error", err)
			continue
		}
		defer file.Close()
		id, err := ih.Add(file)
		if err != nil {
			log.Default().Warn("failed to add image", "file", info.Name(), "error", err)
		} else {
			log.Default().Debug("added image", "file", file.Name(), "id", id)
		}

	}
	return nil
}
package main

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/a-h/templ"
	log "github.com/charmbracelet/log"
	"github.com/johan-st/go-image-server/images"
	components "github.com/johan-st/go-image-server/pages/components"
	"github.com/johan-st/go-image-server/units/size"
	"github.com/johan-st/go-image-server/way"
)

//go:embed pages/assets
var staticFS embed.FS
var (
	darkTheme = components.Theme{
		ColorPrimary:    "#f90",
		ColorSecondary:  "#fa3",
		ColorBackground: "#333",
		ColorText:       "#aaa",
		ColorBorder:     "#666",
		BorderRadius:    "1rem",
	}
	
	metadata = map[string]string{
		"Description": "img.jst.dev is a way for Johan Strand to learn more Go and web development.",
		"Keywords":    "image, hosting",
		"Author":      "Johan Strand",
	}
)


type server struct {
	errorLogger  *log.Logger // *required
	accessLogger *log.Logger // optional

	conf   confHttp
	ih     *images.ImageHandler
	router way.Router

	// TODO: make concurrent safe
	Stats struct {
		StartTime    time.Time
		Requests     int
		Errors       int
		ImagesServed int
	}
}

// Register handlers for routes
func (srv *server) routes() {

	srv.Stats.StartTime = time.Now()

	// Docs / root
	if srv.conf.Docs {
		srv.router.HandleFunc("GET", "", srv.handleDocs())
	}

	// STATIC ASSETS
	srv.router.HandleFunc("GET", "/favicon.ico", srv.handleFavicon())
	srv.router.HandleFunc("GET", "/assets/", srv.handleAssets())

	// API
	srv.router.HandleFunc("GET", "/api/images", srv.handleApiImageGet())
	srv.router.HandleFunc("POST", "/api/images", srv.handleApiImagePost())
	srv.router.HandleFunc("DELETE", "/api/images/:id", srv.handleApiImageDelete())
	srv.router.HandleFunc("*", "/api/", srv.handleNotAllowed())

	// Admin
	srv.router.HandleFunc("GET", "/admin", srv.handleAdminTempl())
	srv.router.HandleFunc("GET", "/admin/:page", srv.handleAdminTempl())
	srv.router.HandleFunc("GET", "/admin/images/:id", srv.handleAdminImage())

	// Serve Images
	srv.router.HandleFunc("GET", "/:id/:preset/", srv.handleImgWithPreset())
	srv.router.HandleFunc("GET", "/:id/", srv.handleImg())

	// 404
	srv.router.NotFound = srv.handleNotFound()
}

// HANDLERS

func (srv *server) handleAdminTempl() http.HandlerFunc {
	// setup
	l := srv.errorLogger.With("handler", "handleAdminTempl")

	// get base css styles
	styles, err := os.ReadFile("pages/assets/admin.css")
	

**Response**:
The presented Go code comprises multiple parts including a main server, a set of HTTP handlers, a set of utility functions, and a script for generating code. It mostly appears to work as expected, but it's important to undertake rigorous testing to uncover potential bugs or edge cases. Here are few areas that need attention:

1. Error handling: There are a few places where error handling could be improved. For example, in `handleApiImagePost` function, if an error occurs while adding image to the handler, there's a use case where function only logs an error without responding resulting in a hanging request. 

2. In `run` function, right after calling `toImagePresets`, there is no error check. This could potentially crash the application due to a nil error. 

3. Inconsistent error reporting: Throughout the code, some functions log errors while others return them. This can make debugging more difficult and can lead to errors being missed. It is a good practice to choose one error handling method and stick with it throughout the codebase.

4. The `/admin/:page` and `/admin/images/:id` routes handlers doesn't seem to validate the input parameters, which can cause potential bugs or security issues.

5. The `loadConfig` attempts to parse a YAML file, but it assumes that the format of the file will always be correct. The program should anticipate and handle possibly malformed configuration files.

6. In `run` function, even after the failure attempts of folder population (via `addFolder`), the code still continues to the execution. This may be designed behavior but in case the folder is critical, this could lead to unexpected results.

7. There doesn’t seem to be any form of authentication on the endpoints. Depending on what this code is being used for, certain routes should be protected like the ones for deletion or creating new entries.

In addition, the code would benefit from more comments, especially for more complex logic. This will be beneficial for maintenance and collaboration purposes. Remember when identifying potential issues with any piece of code it largely depends on the context it is used in, some issues may not apply, while there may be other issues when considering the wider system it is a part of.


