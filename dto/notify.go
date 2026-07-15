package dto

type Notify struct {
	Type          string                   `json:"type"`
	Title         string                   `json:"title"`
	Content       string                   `json:"content"`
	Values        []interface{}            `json:"values"`
	EmailTemplate *EmailTemplateNotifyData `json:"email_template,omitempty"`
}

type EmailTemplateNotifyData struct {
	Event     string            `json:"event"`
	Locale    string            `json:"locale"`
	Variables map[string]string `json:"variables"`
}

const ContentValueParam = "{{value}}"

const (
	NotifyTypeQuotaExceed   = "quota_exceed"
	NotifyTypeChannelUpdate = "channel_update"
	NotifyTypeChannelTest   = "channel_test"
)

func NewNotify(t string, title string, content string, values []interface{}) Notify {
	return Notify{
		Type:    t,
		Title:   title,
		Content: content,
		Values:  values,
	}
}

func (notify Notify) WithEmailTemplate(event, locale string, variables map[string]string) Notify {
	notify.EmailTemplate = &EmailTemplateNotifyData{
		Event:     event,
		Locale:    locale,
		Variables: variables,
	}
	return notify
}
