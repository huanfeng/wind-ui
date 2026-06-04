package widget

import (
	"testing"

	"github.com/huanfeng/wind-ui/core"
)

func TestShowToast(t *testing.T) {
	root := core.NewNode("Root")
	root.SetMeasuredSize(core.Size{Width: 400, Height: 600})
	root.SetBounds(core.Rect{Width: 400, Height: 600})

	toast := ShowToast(root, "Hello World", ToastShort)
	if !toast.IsShowing() {
		t.Error("expected toast to be showing")
	}
	if toast.GetMessage() != "Hello World" {
		t.Errorf("expected 'Hello World', got %q", toast.GetMessage())
	}
	if len(root.Children()) != 1 {
		t.Errorf("expected 1 overlay child, got %d", len(root.Children()))
	}

	toast.Dismiss()
	if toast.IsShowing() {
		t.Error("expected toast not showing after dismiss")
	}
	// Overlay stays in tree but is set to Gone (avoids stale canvas pixels).
	if len(root.Children()) != 1 {
		t.Errorf("expected 1 overlay child (Gone) after dismiss, got %d", len(root.Children()))
	}
	if root.Children()[0].GetVisibility() != core.Gone {
		t.Error("expected overlay to be Gone after dismiss")
	}
}

func TestToastDoubleDismiss(t *testing.T) {
	root := core.NewNode("Root")
	root.SetMeasuredSize(core.Size{Width: 400, Height: 600})
	root.SetBounds(core.Rect{Width: 400, Height: 600})

	toast := ShowToast(root, "Test", ToastShort)
	toast.Dismiss()
	toast.Dismiss() // should not panic
	if toast.IsShowing() {
		t.Error("expected not showing")
	}
}

func TestNewSnackbar(t *testing.T) {
	root := core.NewNode("Root")
	root.SetMeasuredSize(core.Size{Width: 400, Height: 600})
	root.SetBounds(core.Rect{Width: 400, Height: 600})

	clicked := false
	sb := NewSnackbar(root, "File deleted", "UNDO", func() {
		clicked = true
	})

	if sb.actionText != "UNDO" {
		t.Errorf("expected action 'UNDO', got %q", sb.actionText)
	}
	if sb.duration != ToastLong {
		t.Error("expected ToastLong duration for snackbar")
	}

	// Simulate action click
	sb.actionClick()
	if !clicked {
		t.Error("expected action click handler to be called")
	}

	sb.Dismiss()
}

func TestToastDurations(t *testing.T) {
	if ToastShort != 0 {
		t.Errorf("expected ToastShort=0, got %d", ToastShort)
	}
	if ToastLong != 1 {
		t.Errorf("expected ToastLong=1, got %d", ToastLong)
	}
}
