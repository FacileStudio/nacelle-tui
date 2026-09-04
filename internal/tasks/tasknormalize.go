package tasks

import "strings"

func normalizeTasks(list taskList) taskList {
	if list == nil {
		return nil
	}
	normalized := make(taskList, len(list))
	for i, item := range list {
		normalized[i] = normalizeTaskItem(item)
	}
	return normalized
}

func normalizeStepUpdate(step stepUpdate) stepUpdate {
	if step.Status != nil {
		normalizedStatus := normalizeStatus(*step.Status)
		step.Status = &normalizedStatus
	}
	if step.Title != nil {
		normalizedTitle := normalizeTitle(*step.Title)
		step.Title = &normalizedTitle
	}
	return step
}

func normalizeTaskItem(item taskItem) taskItem {
	item.Status = normalizeStatus(item.Status)
	item.Title = normalizeTitle(item.Title)
	return item
}

func normalizeStatus(status string) string {
	if status == "" {
		return statusTodo
	}
	lower := strings.ToLower(status)
	switch lower {
	case "pending", "todo", "to do":
		return statusTodo
	case "in_progress", "in progress", "active", "started":
		return statusActive
	case "completed", "done", "finished":
		return statusDone
	case "blocked", "block":
		return statusBlocked
	case "failed", "fail":
		return statusFailed
	default:
		return status
	}
}

func normalizeTitle(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return "(untitled)"
	}
	return title
}
