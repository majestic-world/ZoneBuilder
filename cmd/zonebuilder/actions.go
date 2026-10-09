package main

import "zonebuilder/internal/locale"

// actionStatus keeps the user-visible result of an action, not its rendered text.
// The editor retains its own message in zoneEditor.LastMessage.
type actionStatus struct {
	message      locale.Message
	water        *waterSummary
	editor       *zoneEditor
	pin          *worstPin
	raw          string
	problemClick bool
}

func action(key string) actionStatus {
	return actionStatus{message: locale.Message{Key: key}}
}

func actionArgs(key string, args map[string]string) actionStatus {
	return actionStatus{message: locale.Message{Key: key, Args: args}}
}
func waterAction(key string, summary *waterSummary) actionStatus {
	return actionStatus{message: locale.Message{Key: key}, water: summary}
}

func actionError(key string, err error, args map[string]string) actionStatus {
	if args == nil {
		args = make(map[string]string, 1)
	}
	args["detail"] = err.Error()
	return actionArgs(key, args)
}

func (s actionStatus) render(lang locale.Language) string {
	if s.water != nil {
		return s.water.render(lang)
	}
	if s.pin != nil {
		return pinStatus(*s.pin, lang)
	}
	if s.editor != nil {
		return s.editor.Status(lang)
	}
	if s.raw != "" {
		return s.raw
	}
	if s.message.Key == "" {
		return ""
	}
	return s.message.Render(lang)
}

func editorResult(text string, editor *zoneEditor) actionStatus {
	if text == "" {
		return actionStatus{}
	}
	if editor.LastMessage.Key == "" {
		return actionStatus{raw: text}
	}
	return actionStatus{message: editor.LastMessage, editor: editor}
}
