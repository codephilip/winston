package scheduler

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// ScheduledTask represents a Claude native scheduled task.
type ScheduledTask struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Agent     string `json:"agent"`
	Schedule  string `json:"schedule"`
	Prompt    string `json:"prompt"`
	SlackChan string `json:"slack_channel,omitempty"`
	Status    string `json:"status"`
}

// CreateNativeSchedule uses the Claude CLI to create a scheduled task.
// This leverages Claude's built-in cloud tasks with Slack connector.
func CreateNativeSchedule(task ScheduledTask) error {
	// Build the prompt that includes Slack posting instructions
	prompt := task.Prompt
	if task.SlackChan != "" {
		prompt = fmt.Sprintf("%s\n\nAfter completing the task, post the results to the Slack channel %s.", prompt, task.SlackChan)
	}

	// Use claude CLI to create the schedule
	// The /schedule command creates a cloud task that runs on Anthropic's infra
	cmd := exec.Command("claude",
		"--print",
		fmt.Sprintf("/schedule %s: %s", task.Schedule, prompt),
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create schedule: %w\noutput: %s", err, string(output))
	}

	return nil
}

// ListNativeSchedules lists all Claude native scheduled tasks.
func ListNativeSchedules() ([]ScheduledTask, error) {
	cmd := exec.Command("claude", "--print", "/schedule list")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to list schedules: %w", err)
	}

	// Parse the output - Claude returns a text list
	// TODO: Parse structured output when available
	lines := strings.Split(string(output), "\n")
	var tasks []ScheduledTask
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			tasks = append(tasks, ScheduledTask{
				Name:   line,
				Status: "active",
			})
		}
	}

	return tasks, nil
}

// DeleteNativeSchedule deletes a Claude native scheduled task.
func DeleteNativeSchedule(taskName string) error {
	cmd := exec.Command("claude", "--print", fmt.Sprintf("/schedule delete %s", taskName))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to delete schedule: %w\noutput: %s", err, string(output))
	}
	return nil
}

// RunNow triggers an immediate run of a scheduled task.
func RunNow(taskName string) (string, error) {
	cmd := exec.Command("claude", "--print", fmt.Sprintf("/schedule run %s", taskName))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to run schedule: %w\noutput: %s", err, string(output))
	}
	return string(output), nil
}

// ToJSON serializes a task for API responses.
func (t ScheduledTask) ToJSON() ([]byte, error) {
	return json.Marshal(t)
}
