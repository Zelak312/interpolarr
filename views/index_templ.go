// Code generated by templ - DO NOT EDIT.

// templ: version: v0.2.778
package views

//lint:file-ignore SA4006 This context is only used if a nested component is present.

import "github.com/a-h/templ"
import templruntime "github.com/a-h/templ/runtime"

import "fmt"

func Index(videosInQueueCount int, activeWorkerCount int) templ.Component {
	return templruntime.GeneratedTemplate(func(templ_7745c5c3_Input templruntime.GeneratedComponentInput) (templ_7745c5c3_Err error) {
		templ_7745c5c3_W, ctx := templ_7745c5c3_Input.Writer, templ_7745c5c3_Input.Context
		if templ_7745c5c3_CtxErr := ctx.Err(); templ_7745c5c3_CtxErr != nil {
			return templ_7745c5c3_CtxErr
		}
		templ_7745c5c3_Buffer, templ_7745c5c3_IsBuffer := templruntime.GetBuffer(templ_7745c5c3_W)
		if !templ_7745c5c3_IsBuffer {
			defer func() {
				templ_7745c5c3_BufErr := templruntime.ReleaseBuffer(templ_7745c5c3_Buffer)
				if templ_7745c5c3_Err == nil {
					templ_7745c5c3_Err = templ_7745c5c3_BufErr
				}
			}()
		}
		ctx = templ.InitializeContext(ctx)
		templ_7745c5c3_Var1 := templ.GetChildren(ctx)
		if templ_7745c5c3_Var1 == nil {
			templ_7745c5c3_Var1 = templ.NopComponent
		}
		ctx = templ.ClearChildren(ctx)
		_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString("<!doctype html><html lang=\"en\"><head><meta charset=\"UTF-8\"><meta name=\"viewport\" content=\"width=device-width, initial-scale=1.0\"><title>Interpolar</title><style>\n                body {\n                    font-family: Arial, sans-serif;\n                    margin: 0;\n                    display: flex;\n                }\n                .sidebar {\n                    width: 200px;\n                    background-color: #2c3e50;\n                    min-height: 100vh;\n                    color: #ecf0f1;\n                    position: fixed;\n                }\n                .sidebar h2 {\n                    text-align: center;\n                    padding: 1rem 0;\n                    background-color: #1a252f;\n                    margin: 0;\n                }\n                .sidebar ul {\n                    list-style: none;\n                    padding: 1rem;\n                }\n                .sidebar ul li {\n                    margin: 1rem 0;\n                }\n                .sidebar ul li a {\n                    color: #ecf0f1;\n                    text-decoration: none;\n                }\n                .main-content {\n                    margin-left: 200px;\n                    padding: 2rem;\n                }\n                .main-content h1 {\n                    margin-bottom: 1rem;\n                }\n                .cards {\n                    display: flex;\n                    gap: 1rem;\n                }\n                .card {\n                    flex: 1;\n                    background-color: #ecf0f1;\n                    padding: 1rem;\n                    border-radius: 5px;\n                }\n                .card h3 {\n                    margin-bottom: 0.5rem;\n                }\n                .activity {\n                    margin-top: 2rem;\n                }\n                .activity h2 {\n                    margin-bottom: 1rem;\n                }\n                .activity ul {\n                    list-style: none;\n                    padding: 0;\n                }\n                .activity ul li {\n                    background-color: #ecf0f1;\n                    margin-bottom: 0.5rem;\n                    padding: 0.5rem;\n                    border-radius: 5px;\n                }\n            </style></head><body><!-- Sidebar Navigation --><div class=\"sidebar\"><h2>Interpolar</h2><ul><li><a href=\"#\">Dashboard</a></li><li><a href=\"#\">Video Queue</a></li><li><a href=\"#\">Workers</a></li><li><a href=\"#\">Settings</a></li></ul></div><!-- Main Content --><div class=\"main-content\"><h1>Dashboard</h1><!-- Summary Cards --><div class=\"cards\"><div class=\"card\"><h3>Videos in Queue</h3><p>")
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		var templ_7745c5c3_Var2 string
		templ_7745c5c3_Var2, templ_7745c5c3_Err = templ.JoinStringErrs(fmt.Sprint(videosInQueueCount))
		if templ_7745c5c3_Err != nil {
			return templ.Error{Err: templ_7745c5c3_Err, FileName: `views/index.templ`, Line: 98, Col: 41}
		}
		_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(templ.EscapeString(templ_7745c5c3_Var2))
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString("</p></div><div class=\"card\"><h3>Active Workers</h3><p>")
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		var templ_7745c5c3_Var3 string
		templ_7745c5c3_Var3, templ_7745c5c3_Err = templ.JoinStringErrs(fmt.Sprint(activeWorkerCount))
		if templ_7745c5c3_Err != nil {
			return templ.Error{Err: templ_7745c5c3_Err, FileName: `views/index.templ`, Line: 102, Col: 40}
		}
		_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(templ.EscapeString(templ_7745c5c3_Var3))
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString("</p></div></div><!-- Recent Activity --><div class=\"activity\"><h2>Recent Activity</h2><ul><li>Video \"Sample1.mp4\" has been processed.</li><li>Worker 3 has started processing \"Sample2.mp4\".</li><li>New video \"Sample3.mp4\" added to the queue.</li><li>Worker 2 has completed a task.</li></ul></div></div></body></html>")
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		return templ_7745c5c3_Err
	})
}

var _ = templruntime.GeneratedTemplate
