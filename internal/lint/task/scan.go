package task

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"awiki/internal/wiki"
)

func ScanContent(contentDir string) (Scan, error) {
	var scan Scan
	err := filepath.WalkDir(contentDir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Ext(path) != ".md" {
			return nil
		}
		page, err := wiki.ParsePage(path, contentDir)
		if err != nil {
			return err
		}
		repoRel := filepath.ToSlash(filepath.Join(filepath.Base(contentDir), page.RelPath))
		if filepath.Base(contentDir) != "content" {
			repoRel = page.RelPath
		}
		info := PageInfo{
			RelPath:     repoRel,
			AbsPath:     page.Path,
			Slug:        page.Slug,
			Type:        page.Type,
			Status:      page.Frontmatter["status"],
			LastUpdated: page.LastUpdated,
			Aliases:     page.Aliases,
		}
		scan.Pages = append(scan.Pages, info)
		scanPage(&scan, page)
		return nil
	})
	return scan, err
}

func scanPage(scan *Scan, page wiki.Page) {
	repoRel := filepath.ToSlash(filepath.Join("content", page.RelPath))
	lines := strings.Split(page.Body, "\n")
	inFence := false
	prevAction := false
	inAgenda := false
	agendaRegion := ""
	for i, line := range lines {
		lineNo := i + len(page.FrontRawLines) + 3
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inFence = !inFence
			prevAction = false
			continue
		}
		if inFence {
			prevAction = false
			continue
		}
		if region, ok := beginAgendaRegion(trimmed); ok {
			inAgenda = true
			agendaRegion = region
			continue
		}
		if inAgenda && strings.HasPrefix(trimmed, "<!-- END agenda:") {
			inAgenda = false
			agendaRegion = ""
			continue
		}
		if inAgenda && !allowedAgendaLine(line) {
			scan.AgendaEdits = append(scan.AgendaEdits, AgendaEdit{File: page.RelPath, Line: lineNo, Region: agendaRegion})
		}
		if strings.HasPrefix(trimmed, ">") {
			prevAction = false
			continue
		}
		if prevAction && len(line) > 0 && (line[0] == ' ' || line[0] == '\t') && strings.TrimSpace(line) != "" {
			scan.Continuations = append(scan.Continuations, Continuation{File: page.RelPath, Line: lineNo})
			continue
		}
		action, ok := ParseActionLine(line)
		if !ok {
			prevAction = false
			continue
		}
		if page.Type == "agenda" {
			prevAction = true
			continue
		}
		action.File = repoRel
		action.AbsPath = page.Path
		action.Line = lineNo
		if page.Type == "project" {
			action.Project = page.Slug
		}
		scan.Actions = append(scan.Actions, action)
		prevAction = true
	}
}

var beginAgendaPattern = regexp.MustCompile(`^<!-- BEGIN agenda:([a-z-]+) -->$`)

func beginAgendaRegion(line string) (string, bool) {
	match := beginAgendaPattern.FindStringSubmatch(line)
	if len(match) == 0 {
		return "", false
	}
	return match[1], true
}

func allowedAgendaLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "<!--") || strings.HasPrefix(trimmed, ">") {
		return true
	}
	if strings.HasPrefix(trimmed, "#") {
		return true
	}
	return actionLinePattern.MatchString(line)
}
