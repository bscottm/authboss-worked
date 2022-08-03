package abossworked

/* "scooter me fecit"

Copyright 2022 B. Scott Michel

This program is free software: you can redistribute it and/or modify it under
the terms of the GNU General Public License as published by the Free Software
Foundation, either version 3 of the License, or (at your option) any later
version.

This program is distributed in the hope that it will be useful, but WITHOUT ANY
WARRANTY; without even the implied warranty of MERCHANTABILITY or FITNESS FOR A
PARTICULAR PURPOSE. See the GNU General Public License for more details.

You should have received a copy of the GNU General Public License along with
this program. If not, see <https://www.gnu.org/licenses/>.
*/

import (
	"context"
	"fmt"
	"html/template"
	"io/fs"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/oxtoacart/bpool"
	"github.com/volatiletech/authboss/v3"
)

const (
	contentTypeText = "text/plain"
	contentTypeHTML = "text/html"

	htmlMasterLayout = "<master layout>"
)

// TemplateState keeps the parsed template and enough state so that the template can be
// hot-(re)loaded if any of its components change.
type TemplateState struct {
	html     *template.Template
	filePath string
	lastMod  time.Time
}

// Templates is a map of all parsed templates.
type Templates struct {
	// Logger
	logger *log.Logger
	// The mapping between an authboss module/page and a HTML template
	templateMap map[string]TemplateState
	// Additional (key, value) data used in the master template
	TemplateData map[string]authboss.HTMLData
	// The master layout template
	masterTemplate TemplateState
	// Fragment templates associated inside the master template (which need to be
	// reloaded if the master template changes.)
	fragmentMap map[string]TemplateState
	// Template directory (when we have to reload the master layout)
	templateDir string
	// Fragment directory (when we have to reload the master layout)
	fragmentDir string
	// Master template file (when we have to reload the master layout)
	masterTemplateFile string
	// Template helper functions
	templateFuncs template.FuncMap
}

// pathFilterFunc filters file names within directory traversals. For example, fragment
// templates start with a leading underscore, so any file that doesn't should not be
// parsed as a template in the corresponding action function.
type pathFilterFunc func(fullPath string, baseName string) bool

// actionFunc is a directory traversal action function -- it's what gets done once we've
// decided we like the file and load the file's content.
type actionFunc func(name string, path string, relPath string, content []byte) error

// Create a pool with 10 buffers
var (
	bufPool = bpool.NewBufferPool(10)

	// These templates are not rendered in the normal way with ExecuteTemplate or
	// with a master layout template.
	nonRenderedTemplates = map[string]string{
		"confirm_txt":  contentTypeText,
		"confirm_html": contentTypeHTML,
		"recover_txt":  contentTypeText,
		"recover_html": contentTypeHTML,
	}
)

// =~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=
// Interface methods for authboss.Render:
// =~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=

// Load templates needed by authboss. The names will be a list of page names
// used by authboss modules, e.g., "login" for the login/user authorization
// module, "register" for the user-initiated account creation module, etc. Load
// will be invoked for each Authboss module, so this function will be invoked
// multiple times.
//
// See Authboss' use cases for the "Pages" that will be rendered for a
// particular module: https://github.com/volatiletech/authboss#use-cases
//
// Note: We've already loaded the templates prior to Authboss calling Load: we
// call TemplateLoader() before we call configureAuthboss(). So, all Load() does
// here is validate that we already loaded the templates that Authboss needs.
func (templates *Templates) Load(names ...string) error {
	if len(templates.templateMap) > 0 {
		for _, name := range names {
			templates.logger.Printf("Templates.Load: Verifying %s", name)
			_, ok := templates.templateMap[name]
			if !ok {
				return fmt.Errorf("Templates.Load: no such template %s loaded", name)
			}
		}

		return nil
	}

	return fmt.Errorf("Template.Load: template map not loaded or incomplete")
}

// Render a specific authboss template; see the notes in Load().
func (templates *Templates) Render(ctx context.Context, page string, data authboss.HTMLData) ([]byte, string, error) {
	templates.logger.Printf("Rendering %s", page)
	// Reload, if necessary:
	err := templates.reloadTemplate(page)
	if err != nil {
		return []byte(fmt.Sprintf("couldn't reload %s", page)), contentTypeText, err
	}

	buf := bufPool.Get()
	defer bufPool.Put(buf)

	tmpl, ok := templates.templateMap[page]
	if !ok {
		errString := fmt.Sprintf("no such HTML template '%s'", page)
		return []byte(errString), contentTypeText, fmt.Errorf(errString)
	}

	contentType := contentTypeHTML
	if specialContent, special := nonRenderedTemplates[page]; !special {
		// Not special.
		// Combine the incoming with the additional static data.
		combined := authboss.NewHTMLData().Merge(data)
		if htmlData, valid := templates.TemplateData[page]; valid {
			combined.Merge(htmlData)
		}

		err = tmpl.html.ExecuteTemplate(buf, htmlMasterLayout, combined)
	} else {
		contentType = specialContent
		err = tmpl.html.Execute(buf, data)
	}

	if err != nil {
		return []byte(fmt.Sprintf("%v", err)), contentTypeText, err
	}

	return buf.Bytes(), contentType, nil
}

// =~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=
// Implementation:
// =~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=~=

/*
TemplateLoader loads and parses the .gohtml template files from templateDir,
collecting the templates in a map. Panics on failure to parse/load anything.

masterTemplate: The overall base HTML container template. "Fragment" templates
are associated with this template first, i.e., the master template is the
aggregate of itself and the partials.

The master template has a magic internal template reference to "content" -- this
is the content of the regular templates that is interpolated when the template
is rendered.

The regular templates are loaded into a clone of the master template, where the
the internal template name "content" references the actual template content.
*/
func TemplateLoader(templateDir, fragmentDir, masterTemplate string, funcs template.FuncMap, logger *log.Logger) (*Templates, error) {
	tpls := &Templates{
		logger:             logger,
		templateMap:        map[string]TemplateState{},
		TemplateData:       map[string]authboss.HTMLData{},
		masterTemplate:     TemplateState{},
		fragmentMap:        map[string]TemplateState{},
		templateDir:        templateDir,
		fragmentDir:        fragmentDir,
		masterTemplateFile: masterTemplate,
		templateFuncs:      funcs,
	}

	err := tpls.loadTemplates()
	if err != nil {
		return nil, err
	}

	return tpls, nil
}

// loadTemplates does all of the heaving lifting for template loading, parsing, etc.
func (templates *Templates) loadTemplates() error {
	masterTmplPath := filepath.Join(templates.templateDir, templates.masterTemplateFile)
	b, err := ioutil.ReadFile(masterTmplPath)
	if err != nil {
		return fmt.Errorf("could not load master template: %v", err)
	}

	masterTpl, err := template.New(htmlMasterLayout).Funcs(templates.templateFuncs).Parse(string(b))
	if err != nil {
		return fmt.Errorf("failed to parse master template: %v", err)
	}

	// Can ignore the error because we already know it exists...
	masterLastMod, _ := os.Stat(masterTmplPath)

	templates.masterTemplate = TemplateState{
		html:     masterTpl,
		filePath: masterTmplPath,
		lastMod:  masterLastMod.ModTime(),
	}

	if len(templates.fragmentDir) > 0 {
		// Traverse the fragment templates directory, parsing the encountered fragment template,
		// associating each fragment template in the master layout template.
		//
		// Ignores templates whose file name doesn't start with "_" or ends in ".gohtml"
		err = filepath.WalkDir(templates.fragmentDir, walkFunction(templates.templateDir,
			// Filter function:
			func(_fullPath string, baseName string) bool {
				return strings.HasPrefix(baseName, "_") && strings.HasSuffix(baseName, ".gohtml")
			},
			// Action function:
			func(name, path, relPath string, content []byte) error {
				// Nest the fragment within the master template:
				fragHTML, err := masterTpl.New(name).Parse(string(content))
				if err != nil {
					return fmt.Errorf("failed to parse fragment: (%s): %v", relPath, err)
				}

				templates.fragmentMap[name] = TemplateState{
					html:     fragHTML,
					filePath: path,
					lastMod:  getModTime(path),
				}

				templates.logger.Printf("Loaded fragment template '%s' from %s", name, path)
				return nil
			}))

		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to load fragment templates: %v", err)
		}
	}

	// Then sweep through templateDir for the regular templates. Ingore files starting with "_"
	// or not ending in ".gohtml"
	masterTemplatePath := filepath.Join(templates.templateDir, templates.masterTemplateFile)
	err = filepath.WalkDir(templates.templateDir, walkFunction(templates.templateDir,
		// Path name filter function:
		func(fullPath, baseName string) bool {
			return fullPath != masterTemplatePath && !strings.HasPrefix(baseName, "_") && strings.HasSuffix(baseName, ".gohtml")
		},
		// Action function:
		func(name, path, relPath string, content []byte) error {
			clone, err := masterTpl.Clone()
			if err != nil {
				return fmt.Errorf("failed to clone layout: %w", err)
			}

			t, err := clone.New("content").Parse(string(content))
			if err != nil {
				return fmt.Errorf("failed to parse template (%s): %w", relPath, err)
			}

			templates.templateMap[name] = TemplateState{
				html:     t,
				filePath: path,
				lastMod:  getModTime(path),
			}

			templates.logger.Printf("Loaded HTML template '%s' from %s", name, path)

			return nil
		}))

	if err != nil {
		return fmt.Errorf("failed to load templates: %v", err)
	}

	return nil
}

func (templates *Templates) reloadTemplate(key string) error {
	// Check if the master template has changed.
	master, err := os.Stat(templates.masterTemplate.filePath)
	if err != nil {
		return fmt.Errorf("master layout template %s disappeared (%w)", templates.masterTemplate.filePath, err)
	}

	var fragMod bool = false

	for frag := range templates.fragmentMap {
		if !fragMod {
			fragInfo, err := os.Stat(templates.fragmentMap[frag].filePath)
			if err != nil {
				return fmt.Errorf("could not stat fragment template %s (%w)", templates.fragmentMap[frag].filePath, err)
			}
			if fragInfo.ModTime().After(templates.fragmentMap[frag].lastMod) {
				fragMod = true
			}
		}
	}

	if master.ModTime().After(templates.masterTemplate.lastMod) || fragMod {
		// Reload the master, fragments and the templates. Everything.
		err := templates.loadTemplates()
		if err != nil {
			return err
		}

		templates.logger.Printf("Reloaded all templates.")
		return nil
	}

	tmpl, ok := templates.templateMap[key]
	if !ok {
		return fmt.Errorf("missing template %v in reloadTemplate", key)
	}

	fsinfo, err := os.Stat(tmpl.filePath)
	if err != nil {
		return err
	}

	if fsinfo.ModTime().After(tmpl.lastMod) {
		// Hot reload!
		content, _ := ioutil.ReadFile(tmpl.filePath)

		clone, err := templates.masterTemplate.html.Clone()
		if err != nil {
			return fmt.Errorf("failed to clone layout: %w", err)
		}

		t, err := clone.New("content").Parse(string(content))
		if err != nil {
			return fmt.Errorf("failed to parse template (%s): %w", tmpl.filePath, err)
		}

		templates.templateMap[key] = TemplateState{
			html:     t,
			filePath: tmpl.filePath,
			lastMod:  getModTime(tmpl.filePath),
		}

		templates.logger.Printf("Reloaded HTML template '%s' from %s", key, tmpl.filePath)
	}
	return nil
}

func removeExtension(path string) string {
	dot := strings.Index(path, ".")
	if dot >= 0 {
		return path[:dot]
	}
	return path
}

func getModTime(path string) time.Time {
	lastMod, _ := os.Stat(path)
	return lastMod.ModTime()
}

func walkFunction(templateDir string, pathFilter pathFilterFunc, action actionFunc) fs.WalkDirFunc {
	return func(path string, info fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		if !pathFilter(path, filepath.Base(path)) {
			return nil
		}

		rel, err := filepath.Rel(templateDir, path)
		if err != nil {
			return fmt.Errorf("could not create relative path: %v", err)
		}

		b, err := ioutil.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to load partial (%s): %v", rel, err)
		}

		return action(removeExtension(filepath.Base(rel)), path, rel, b)
	}
}
