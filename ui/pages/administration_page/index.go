package administration_page

import (
	"github.com/a-h/templ"
	administration_page_ui "github.com/yazmeyaa/hosthalla/ui/pages/administration_page/ui"
	"github.com/yazmeyaa/hosthalla/ui/shared/ui/badge"
	"github.com/yazmeyaa/hosthalla/ui/shared/ui/card"
)

type AdministrationNotice = administration_page_ui.AdministrationNotice
type AdministrationPageProps = administration_page_ui.AdministrationPageProps

var AdministrationPage = administration_page_ui.AdministrationPage
var AdministrationPageContent = administration_page_ui.AdministrationPageContent

func CSSClasses() []templ.CSSClass {
	classes := administration_page_ui.CSSClasses()
	classes = append(classes, badge.CSSClasses()...)
	classes = append(classes, card.CSSClasses()...)
	return classes
}
