package help_page

import (
	"github.com/a-h/templ"
	help_page_ui "github.com/yazmeyaa/hosthalla/ui/pages/help_page/ui"
	"github.com/yazmeyaa/hosthalla/ui/shared/ui/callout"
	"github.com/yazmeyaa/hosthalla/ui/shared/ui/codeblock"
)

type HelpPageProps = help_page_ui.HelpPageProps

var HelpPage = help_page_ui.HelpPage
var HelpPageContent = help_page_ui.HelpPageContent

func CSSClasses() []templ.CSSClass {
	classes := help_page_ui.CSSClasses()
	classes = append(classes, callout.CSSClasses()...)
	classes = append(classes, codeblock.CSSClasses()...)
	return classes
}
