// Full Showcase — Comprehensive Regression Demo
//
// Covers ALL widgets across Phase 1–4 in a single window with tab navigation.
// If this demo works correctly by manual inspection, the engine has no regression.
//
// Tabs: Basic | Input | Lists | Layout | Dialog
//
// Usage:
//
//	go run ./examples/fullshowcase              # windowed mode
//	go run ./examples/fullshowcase --screenshot  # render all tabs to PNG files
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"runtime"

	"github.com/huanfeng/wind-ui/app"
	"github.com/huanfeng/wind-ui/core"
	"github.com/huanfeng/wind-ui/layout"
	"github.com/huanfeng/wind-ui/platform"
	"github.com/huanfeng/wind-ui/render/freetype"
	"github.com/huanfeng/wind-ui/render/gg"
	"github.com/huanfeng/wind-ui/widget"
)

func main() {
	if hasFlag("--screenshot") {
		runScreenshot()
		return
	}
	runWindowed()
}

// ============================================================
// Windowed mode
// ============================================================

func runWindowed() {
	application := app.NewApplication()

	window, err := application.CreateWindow(platform.WindowOptions{
		Title:     "Wind UI — Full Showcase",
		Width:     520,
		Height:    680,
		Resizable: true,
	})
	if err != nil {
		panic(fmt.Sprintf("failed to create window: %v", err))
	}

	root := buildUI(application, window)
	window.SetContentView(root)
	window.Center()
	window.Show()

	attachNativeEdits(root, application, window)
	application.Run()
}

// ============================================================
// Screenshot mode — offscreen rendering, no window needed
// ============================================================

func runScreenshot() {
	tr := freetype.NewFreeTypeTextRenderer()
	defer tr.Close()
	tm := core.NewTextMeasurer(tr)

	width, height := 520, 680
	tabFileNames := []string{"basic", "input", "lists", "layout", "dialog"}

	outDir := outputDir()
	os.MkdirAll(outDir, 0o755)
	fmt.Printf("Rendering %d tab screenshots to %s\n", len(tabFileNames), outDir)

	for i, name := range tabFileNames {
		root := buildTabPage(i, tr, width, height)
		if root == nil {
			fmt.Printf("  SKIP: %s (nil root)\n", name)
			continue
		}
		root.SetData("textMeasurer", tm)

		// Layout pass
		wSpec := core.MeasureSpec{Mode: core.MeasureModeExact, Size: float64(width)}
		hSpec := core.MeasureSpec{Mode: core.MeasureModeExact, Size: float64(height)}
		layout.MeasureChild(root, wSpec, hSpec)
		if l := root.GetLayout(); l != nil {
			l.Arrange(root, core.Rect{Width: float64(width), Height: float64(height)})
		}
		root.SetBounds(core.Rect{Width: float64(width), Height: float64(height)})

		// Paint pass
		canvas := gg.NewGGCanvas(width, height, tr)
		app.PaintNode(root, canvas)
		img := canvas.Target()

		// Save PNG
		outPath := filepath.Join(outDir, fmt.Sprintf("tab_%s.png", name))
		if err := savePNG(outPath, img); err != nil {
			fmt.Printf("  ERROR %s: %v\n", name, err)
			continue
		}
		fmt.Printf("  OK: %s (%dx%d)\n", outPath, width, height)
	}

	fmt.Println("Done. Visually inspect the PNGs for regressions.")
}

// buildTabPage builds a single tab page for offscreen rendering.
// It wraps the page in a root LinearLayout with toolbar + status + tab bar
// to simulate the real windowed layout.
func buildTabPage(index int, tr core.TextRenderer, width, height int) *core.Node {
	// Root
	root := core.NewNode("Root")
	root.SetLayout(&layout.LinearLayout{Orientation: layout.Vertical})
	root.SetStyle(&core.Style{
		Width:           core.Dimension{Unit: core.DimensionMatchParent},
		Height:          core.Dimension{Unit: core.DimensionMatchParent},
		BackgroundColor: bgColor("#FAFAFA"),
	})
	root.SetPainter(&bgPainter{})

	// Toolbar
	toolbar := widget.NewToolbar("Wind UI Full Showcase")
	toolbar.SetSubtitle("Regression Test — All Widgets")
	toolbar.Node().SetStyle(&core.Style{
		Width:           core.Dimension{Unit: core.DimensionMatchParent},
		Height:          dimVal(50),
		BackgroundColor: bgColor("#1565C0"),
		TextColor:       core.ParseColor("#FFFFFF"),
		FontSize:        18,
	})

	// Status bar
	statusTV := widget.NewTextView("Tab: " + tabNames[index])
	statusTV.Node().SetStyle(&core.Style{
		Width:     core.Dimension{Unit: core.DimensionMatchParent},
		Height:    dimVal(22),
		FontSize:  11,
		TextColor: core.ParseColor("#FF5722"),
	})
	statusTV.Node().GetStyle().Gravity = core.GravityCenter

	// Tab bar (visual only)
	tabBar := core.NewNode("TabBar")
	tabBar.SetLayout(&layout.LinearLayout{Orientation: layout.Horizontal})
	tabBar.SetStyle(&core.Style{
		Width:           core.Dimension{Unit: core.DimensionMatchParent},
		Height:          dimVal(40),
		BackgroundColor: bgColor("#0D47A1"),
	})
	tabBar.SetPainter(&bgPainter{})
	for i, name := range tabNames {
		tab := widget.NewTextView(name)
		tc := core.ParseColor("#90CAF9")
		if i == index {
			tc = core.ParseColor("#FFFFFF")
		}
		tab.Node().SetStyle(&core.Style{
			Width:           dimWeight(1),
			Height:          dimVal(40),
			FontSize:        12,
			TextColor:       tc,
			BackgroundColor: bgColor("#0D47A1"),
		})
		tab.Node().GetStyle().Weight = 1
		tab.Node().GetStyle().Gravity = core.GravityCenter
		tab.Node().SetPainter(&bgPainter{})
		tabBar.AddChild(tab.Node())
	}

	// Content area
	dummyWindow := &dummyWindow{dpi: 96}
	us := func(string) {} // no-op status updater for screenshot
	page := buildPageByIndex(index, nil, dummyWindow, us)
	if page == nil {
		return nil
	}

	// Set the page to fill remaining space
	page.Node().SetStyle(&core.Style{
		Width:  core.Dimension{Unit: core.DimensionMatchParent},
		Height: dimWeight(1),
	})
	page.Node().GetStyle().Weight = 1

	root.AddChild(toolbar.Node())
	root.AddChild(statusTV.Node())
	root.AddChild(tabBar)
	root.AddChild(page.Node())

	return root
}

// dummyWindow satisfies platform.Window for offscreen rendering.
type dummyWindow struct{ dpi float64 }

func (d *dummyWindow) SetContentView(_ *core.Node)            {}
func (d *dummyWindow) SetTitle(_ string)                      {}
func (d *dummyWindow) SetIcon(_ *core.ImageResource)          {}
func (d *dummyWindow) Show()                                  {}
func (d *dummyWindow) Hide()                                  {}
func (d *dummyWindow) Close()                                 {}
func (d *dummyWindow) Minimize()                              {}
func (d *dummyWindow) Maximize()                              {}
func (d *dummyWindow) Restore()                               {}
func (d *dummyWindow) SetSize(_, _ int)                       {}
func (d *dummyWindow) SetPosition(_, _ int)                   {}
func (d *dummyWindow) Center()                                {}
func (d *dummyWindow) IsVisible() bool                        { return false }
func (d *dummyWindow) IsFocused() bool                        { return false }
func (d *dummyWindow) GetSize() core.Size                     { return core.Size{} }
func (d *dummyWindow) GetPosition() core.Point                { return core.Point{} }
func (d *dummyWindow) GetDPI() float64                        { return d.dpi }
func (d *dummyWindow) SetOnClose(_ func() bool)               {}
func (d *dummyWindow) SetOnResize(_ func(w, h int))           {}
func (d *dummyWindow) SetOnDPIChanged(_ func(dpi float64))    {}
func (d *dummyWindow) SetOnFocusChanged(_ func(focused bool)) {}
func (d *dummyWindow) NativeHandle() uintptr                  { return 0 }
func (d *dummyWindow) Invalidate()                            {}
func (d *dummyWindow) InvalidateRect(_ core.Rect)             {}
func (d *dummyWindow) StartAnimator(_ *core.ValueAnimator)    {}
func (d *dummyWindow) RequestFrame()                          {}

// buildPageByIndex dispatches to the correct page builder.
func buildPageByIndex(index int, _ *app.Application, window platform.Window, updateStatus func(string)) core.View {
	a := &showcasePagerAdapter{window: window, updateStatus: updateStatus}
	switch index {
	case 0:
		return a.buildBasicPage()
	case 1:
		return a.buildInputPage()
	case 2:
		return a.buildListsPage()
	case 3:
		return a.buildLayoutPage()
	case 4:
		return a.buildDialogPage()
	default:
		return nil
	}
}

// ============================================================
// UI Builder (shared between windowed and screenshot modes)
// ============================================================

func buildUI(application *app.Application, window platform.Window) *core.Node {
	// Root vertical layout
	root := newVNode("Root", core.DimensionMatchParent, core.DimensionMatchParent)
	root.GetStyle().BackgroundColor = bgColor("#FAFAFA")

	// ---- Toolbar ----
	toolbar := widget.NewToolbar("Wind UI Full Showcase")
	toolbar.SetSubtitle("Regression Test — All Widgets")
	toolbar.Node().SetStyle(&core.Style{
		Width:           dim(core.DimensionMatchParent),
		Height:          dimVal(50),
		BackgroundColor: bgColor("#1565C0"),
		TextColor:       core.ParseColor("#FFFFFF"),
		FontSize:        18,
	})
	toolbar.AddAction(widget.ActionItem{
		ID: "info", Title: "Info",
		OnClick: func() {
			widget.ShowToast(root, "Wind UI Full Showcase v1.0", widget.ToastShort)
			window.Invalidate()
		},
	})

	// ---- Status bar ----
	statusTV := widget.NewTextView("Interact with widgets to test")
	statusTV.SetId("status")
	statusTV.Node().SetStyle(&core.Style{
		Width:     dim(core.DimensionMatchParent),
		Height:    dimVal(22),
		FontSize:  11,
		TextColor: core.ParseColor("#FF5722"),
	})
	statusTV.Node().GetStyle().Gravity = core.GravityCenter

	updateStatus := func(msg string) {
		statusTV.SetText(msg)
		window.Invalidate()
		fmt.Println(msg)
	}

	// ---- TabLayout ----
	tabLayout := widget.NewTabLayout()
	tabLayout.Node().SetStyle(&core.Style{
		Width:           dim(core.DimensionMatchParent),
		Height:          dimVal(40),
		BackgroundColor: bgColor("#0D47A1"),
		TextColor:       core.ParseColor("#FFFFFF"),
		FontSize:        12,
	})

	tabNames := []string{"Basic", "Input", "Lists", "Layout", "Dialog"}
	for _, name := range tabNames {
		tabLayout.AddTab(widget.Tab{Text: name})
	}

	// ---- ViewPager ----
	viewPager := widget.NewViewPager()
	viewPager.Node().SetStyle(&core.Style{
		Width:  dim(core.DimensionMatchParent),
		Height: dimWeight(1),
	})
	viewPager.Node().GetStyle().Weight = 1

	viewPager.SetAdapter(&showcasePagerAdapter{
		app:          application,
		window:       window,
		updateStatus: updateStatus,
	})
	viewPager.SetupWithTabLayout(tabLayout)
	viewPager.SetOnPageChangedListener(func(idx int) {
		if idx < len(tabNames) {
			updateStatus(fmt.Sprintf("Tab: %s", tabNames[idx]))
		}
	})

	// ---- Assemble ----
	root.AddChild(toolbar.Node())
	root.AddChild(statusTV.Node())
	root.AddChild(tabLayout.Node())
	root.AddChild(viewPager.Node())

	return root
}

// ============================================================
// PagerAdapter
// ============================================================

type showcasePagerAdapter struct {
	app          *app.Application
	window       platform.Window
	updateStatus func(string)
}

func (a *showcasePagerAdapter) GetCount() int { return 5 }

func (a *showcasePagerAdapter) GetPageTitle(i int) string {
	return []string{"Basic", "Input", "Lists", "Layout", "Dialog"}[i]
}

func (a *showcasePagerAdapter) CreatePage(index int) core.View {
	return buildPageByIndex(index, a.app, a.window, a.updateStatus)
}

// ============================================================
// Tab 0: Basic — TextView, Button, Divider, weight layout, FrameLayout
// ============================================================

func (a *showcasePagerAdapter) buildBasicPage() core.View {
	scroll := widget.NewScrollView()
	scroll.Node().SetStyle(&core.Style{
		Width:  dim(core.DimensionMatchParent),
		Height: dim(core.DimensionMatchParent),
	})

	page := newVPage()
	scroll.Node().AddChild(page)

	// -- Section: TextView --
	page.AddChild(sectionLabel("TextView"))
	page.AddChild(bodyText("This is a normal TextView with default styling."))
	tvLarge := widget.NewTextView("Large centered text (20dp)")
	tvLarge.Node().SetStyle(&core.Style{
		Width:     dim(core.DimensionMatchParent),
		Height:    dim(core.DimensionWrapContent),
		FontSize:  20,
		TextColor: core.ParseColor("#1565C0"),
	})
	tvLarge.Node().GetStyle().Gravity = core.GravityCenter
	page.AddChild(tvLarge.Node())

	// -- Section: Buttons --
	page.AddChild(sectionLabel("Buttons"))
	btnRow := newHNode("BtnRow", 8)
	btnPrimary := widget.NewButton("Primary", nil)
	btnPrimary.Node().SetStyle(&core.Style{
		Width:           dimWeight(1),
		Height:          dimVal(40),
		BackgroundColor: bgColor("#1976D2"),
		TextColor:       core.ParseColor("#FFFFFF"),
		CornerRadius:    4,
		FontSize:        14,
	})
	btnPrimary.Node().GetStyle().Weight = 1
	btnPrimary.SetOnClickListener(func(_ core.View) { a.updateStatus("Button: Primary clicked") })

	btnSecondary := widget.NewButton("Secondary", nil)
	btnSecondary.Node().SetStyle(&core.Style{
		Width:           dimWeight(1),
		Height:          dimVal(40),
		BackgroundColor: bgColor("#757575"),
		TextColor:       core.ParseColor("#FFFFFF"),
		CornerRadius:    4,
		FontSize:        14,
	})
	btnSecondary.Node().GetStyle().Weight = 1
	btnSecondary.SetOnClickListener(func(_ core.View) { a.updateStatus("Button: Secondary clicked") })

	btnRow.AddChild(btnPrimary.Node())
	btnRow.AddChild(btnSecondary.Node())
	page.AddChild(btnRow)

	btnDisabled := widget.NewButton("Disabled Button", nil)
	btnDisabled.Node().SetStyle(&core.Style{
		Width:           dim(core.DimensionMatchParent),
		Height:          dimVal(36),
		BackgroundColor: bgColor("#BDBDBD"),
		TextColor:       core.ParseColor("#9E9E9E"),
		CornerRadius:    4,
		FontSize:        14,
	})
	page.AddChild(btnDisabled.Node())

	// -- Section: Divider --
	page.AddChild(sectionLabel("Divider"))
	page.AddChild(bodyText("Above line"))
	page.AddChild(widget.NewDivider().Node())
	page.AddChild(bodyText("Below line"))

	// -- Section: Weight layout --
	page.AddChild(sectionLabel("Weight Layout (LinearLayout weight)"))
	weightRow := newHNode("WeightRow", 4)
	for i, ratio := range []string{"1", "2", "1"} {
		tv := widget.NewTextView("w=" + ratio)
		w := 1.0
		if ratio == "2" {
			w = 2.0
		}
		tv.Node().SetStyle(&core.Style{
			Width:           dimWeight(1),
			Height:          dimVal(36),
			FontSize:        12,
			TextColor:       core.ParseColor("#FFFFFF"),
			BackgroundColor: bgColor([]string{"#1976D2", "#388E3C", "#F57C00"}[i]),
			CornerRadius:    4,
		})
		tv.Node().GetStyle().Weight = w
		tv.Node().GetStyle().Gravity = core.GravityCenter
		weightRow.AddChild(tv.Node())
	}
	page.AddChild(weightRow)

	// -- Section: FrameLayout overlay --
	page.AddChild(sectionLabel("FrameLayout (overlay stacking)"))
	frame := core.NewNode("FrameLayout")
	frame.SetLayout(&layout.FrameLayout{})
	frame.SetStyle(&core.Style{
		Width:           dim(core.DimensionMatchParent),
		Height:          dimVal(80),
		BackgroundColor: bgColor("#E3F2FD"),
		CornerRadius:    8,
	})
	frame.SetPainter(&bgPainter{})
	bg := widget.NewTextView("")
	bg.Node().SetStyle(&core.Style{
		Width:           dim(core.DimensionMatchParent),
		Height:          dimVal(80),
		BackgroundColor: bgColor("#E3F2FD"),
		CornerRadius:    8,
	})
	overlay := widget.NewTextView("Overlaid text in FrameLayout")
	overlay.Node().SetStyle(&core.Style{
		Width:     dim(core.DimensionMatchParent),
		Height:    dimVal(80),
		FontSize:  14,
		TextColor: core.ParseColor("#1565C0"),
	})
	overlay.Node().GetStyle().Gravity = core.GravityCenter
	frame.AddChild(bg.Node())
	frame.AddChild(overlay.Node())
	page.AddChild(frame)

	return scroll
}

// ============================================================
// Tab 1: Input — EditText, CheckBox, RadioButton, Switch, Spinner, SeekBar, ProgressBar
// ============================================================

func (a *showcasePagerAdapter) buildInputPage() core.View {
	scroll := widget.NewScrollView()
	scroll.Node().SetStyle(&core.Style{
		Width:  dim(core.DimensionMatchParent),
		Height: dim(core.DimensionMatchParent),
	})

	page := newVPage()
	scroll.Node().AddChild(page)

	// -- EditText (native, attached later) --
	page.AddChild(sectionLabel("EditText"))
	etName := widget.NewEditText("Name")
	etName.SetId("et_name")
	etName.Node().GetStyle().Width = dim(core.DimensionMatchParent)
	etName.Node().GetStyle().Height = dimVal(36)
	page.AddChild(etName.Node())

	etPassword := widget.NewEditText("Password")
	etPassword.SetId("et_password")
	etPassword.Node().GetStyle().Width = dim(core.DimensionMatchParent)
	etPassword.Node().GetStyle().Height = dimVal(36)
	page.AddChild(etPassword.Node())

	// -- CheckBox --
	page.AddChild(sectionLabel("CheckBox"))
	cb1 := widget.NewCheckBox("I agree to the terms")
	cb1.Node().SetStyle(&core.Style{
		Width:  dim(core.DimensionMatchParent),
		Height: dim(core.DimensionWrapContent),
	})
	cb1.SetOnCheckedChanged(func(checked bool) {
		a.updateStatus(fmt.Sprintf("CheckBox: agreed=%v", checked))
	})
	page.AddChild(cb1.Node())

	cb2 := widget.NewCheckBox("Subscribe to newsletter (checked)")
	cb2.SetChecked(true)
	cb2.Node().SetStyle(&core.Style{
		Width:  dim(core.DimensionMatchParent),
		Height: dim(core.DimensionWrapContent),
	})
	page.AddChild(cb2.Node())

	// -- RadioButton / RadioGroup --
	page.AddChild(sectionLabel("RadioGroup"))
	rg := widget.NewRadioGroup()
	rg.Node().SetStyle(&core.Style{
		Width:  dim(core.DimensionMatchParent),
		Height: dim(core.DimensionWrapContent),
	})
	for _, size := range []string{"Small", "Medium", "Large"} {
		rb := widget.NewRadioButton(size)
		rb.Node().SetStyle(&core.Style{
			Width:  dim(core.DimensionMatchParent),
			Height: dim(core.DimensionWrapContent),
		})
		rg.RegisterButton(rb)
	}
	rg.SetOnChanged(func(idx int) {
		sizes := []string{"Small", "Medium", "Large"}
		if idx < len(sizes) {
			a.updateStatus(fmt.Sprintf("Radio: %s selected", sizes[idx]))
		}
	})
	page.AddChild(rg.Node())

	// -- Switch --
	page.AddChild(sectionLabel("Switch"))
	swRow := newHNode("SwRow", 8)
	swLabel := widget.NewTextView("Dark Mode")
	swLabel.Node().SetStyle(&core.Style{
		Width:     dimWeight(1),
		Height:    dim(core.DimensionWrapContent),
		FontSize:  14,
		TextColor: core.ParseColor("#333333"),
	})
	swLabel.Node().GetStyle().Weight = 1
	sw := widget.NewSwitch()
	sw.Node().SetStyle(&core.Style{
		Width:  dim(core.DimensionWrapContent),
		Height: dim(core.DimensionWrapContent),
	})
	sw.SetOnChanged(func(on bool) {
		a.updateStatus(fmt.Sprintf("Switch: dark=%v", on))
	})
	swRow.AddChild(swLabel.Node())
	swRow.AddChild(sw.Node())
	page.AddChild(swRow)

	// -- Spinner --
	page.AddChild(sectionLabel("Spinner"))
	spinner := widget.NewSpinner([]string{"Option A", "Option B", "Option C", "Option D"})
	spinner.Node().SetStyle(&core.Style{
		Width:           dim(core.DimensionMatchParent),
		Height:          dimVal(38),
		FontSize:        14,
		TextColor:       core.ParseColor("#212121"),
		BorderColor:     core.ParseColor("#BDBDBD"),
		BorderWidth:     1,
		CornerRadius:    4,
		BackgroundColor: bgColor("#FFFFFF"),
	})
	spinner.SetOnItemSelectedListener(func(idx int, item string) {
		a.updateStatus(fmt.Sprintf("Spinner: %s", item))
	})
	page.AddChild(spinner.Node())

	// -- SeekBar --
	page.AddChild(sectionLabel("SeekBar"))
	seekBar := widget.NewSeekBar(0.5)
	seekBar.Node().SetStyle(&core.Style{
		Width:  dim(core.DimensionMatchParent),
		Height: dimVal(28),
	})
	seekVal := widget.NewTextView("50%")
	seekVal.Node().SetStyle(&core.Style{
		Width:     dim(core.DimensionMatchParent),
		Height:    dim(core.DimensionWrapContent),
		FontSize:  12,
		TextColor: core.ParseColor("#333333"),
	})
	seekVal.Node().GetStyle().Gravity = core.GravityCenter
	seekBar.SetOnProgressChangedListener(func(p float64) {
		seekVal.SetText(fmt.Sprintf("%.0f%%", p*100))
		a.updateStatus(fmt.Sprintf("SeekBar: %.0f%%", p*100))
	})
	page.AddChild(seekBar.Node())
	page.AddChild(seekVal.Node())

	// -- ProgressBar --
	page.AddChild(sectionLabel("ProgressBar"))
	pb := widget.NewProgressBar()
	pb.SetProgress(0.6)
	pb.Node().SetStyle(&core.Style{
		Width:  dim(core.DimensionMatchParent),
		Height: dim(core.DimensionWrapContent),
	})
	page.AddChild(pb.Node())
	btnProg := widget.NewButton("Add 10%", nil)
	btnProg.Node().SetStyle(&core.Style{
		Width:           dimVal(100),
		Height:          dimVal(32),
		BackgroundColor: bgColor("#1976D2"),
		TextColor:       core.ParseColor("#FFFFFF"),
		CornerRadius:    4,
		FontSize:        12,
	})
	btnProg.SetOnClickListener(func(_ core.View) {
		p := pb.GetProgress() + 0.1
		if p > 1.0 {
			p = 0.0
		}
		pb.SetProgress(p)
		a.updateStatus(fmt.Sprintf("Progress: %.0f%%", p*100))
		a.window.Invalidate()
	})
	page.AddChild(btnProg.Node())

	return scroll
}

// ============================================================
// Tab 2: Lists — RecyclerView, TreeView
// ============================================================

func (a *showcasePagerAdapter) buildListsPage() core.View {
	scroll := widget.NewScrollView()
	scroll.Node().SetStyle(&core.Style{
		Width:  dim(core.DimensionMatchParent),
		Height: dim(core.DimensionMatchParent),
	})

	page := newVPage()
	scroll.Node().AddChild(page)

	// -- RecyclerView --
	page.AddChild(sectionLabel("RecyclerView (30 items)"))
	rv := widget.NewRecyclerView(44)
	rv.Node().SetStyle(&core.Style{
		Width:           dim(core.DimensionMatchParent),
		Height:          dimVal(220),
		BackgroundColor: bgColor("#FFFFFF"),
		BorderColor:     core.ParseColor("#E0E0E0"),
		BorderWidth:     1,
		CornerRadius:    4,
	})
	rv.SetAdapter(&listAdapter{count: 30})
	rv.SetOnItemClickListener(func(pos int) {
		a.updateStatus(fmt.Sprintf("RecyclerView: item %d", pos+1))
	})
	page.AddChild(rv.Node())

	// -- TreeView --
	page.AddChild(sectionLabel("TreeView (expand/collapse)"))
	tv := widget.NewTreeView()
	tv.Node().SetStyle(&core.Style{
		Width:           dim(core.DimensionMatchParent),
		Height:          dimVal(280),
		BackgroundColor: bgColor("#FAFAFA"),
		TextColor:       core.ParseColor("#333333"),
		FontSize:        13,
		BorderColor:     core.ParseColor("#E0E0E0"),
		BorderWidth:     1,
		CornerRadius:    4,
	})

	src := &widget.TreeNode{Text: "src", Expanded: true}
	src.AddChild(&widget.TreeNode{Text: "main.go"})
	src.AddChild(&widget.TreeNode{Text: "app/"})
	pkg := &widget.TreeNode{Text: "core", Expanded: true}
	pkg.AddChild(&widget.TreeNode{Text: "node.go"})
	pkg.AddChild(&widget.TreeNode{Text: "event.go"})
	pkg.AddChild(&widget.TreeNode{Text: "canvas.go"})
	src.AddChild(pkg)

	tests := &widget.TreeNode{Text: "tests", Expanded: false}
	tests.AddChild(&widget.TreeNode{Text: "widget_test.go"})
	tests.AddChild(&widget.TreeNode{Text: "layout_test.go"})

	docs := &widget.TreeNode{Text: "docs", Expanded: false}
	docs.AddChild(&widget.TreeNode{Text: "README.md"})
	docs.AddChild(&widget.TreeNode{Text: "ARCHITECTURE.md"})

	tv.SetRoots([]*widget.TreeNode{src, tests, docs})
	tv.SetOnNodeSelectedListener(func(n *widget.TreeNode) {
		a.updateStatus(fmt.Sprintf("TreeView: '%s'", n.Text))
	})
	page.AddChild(tv.Node())

	return scroll
}

// ============================================================
// Tab 3: Layout — GridLayout, FlexLayout, SplitPane
// ============================================================

func (a *showcasePagerAdapter) buildLayoutPage() core.View {
	scroll := widget.NewScrollView()
	scroll.Node().SetStyle(&core.Style{
		Width:  dim(core.DimensionMatchParent),
		Height: dim(core.DimensionMatchParent),
	})

	page := newVPage()
	scroll.Node().AddChild(page)

	// -- GridLayout --
	page.AddChild(sectionLabel("GridLayout — 3 columns"))
	gridNode := core.NewNode("GridLayout")
	gridNode.SetLayout(&layout.GridLayout{ColumnCount: 3, Spacing: 8})
	gridNode.SetStyle(&core.Style{
		Width:           dim(core.DimensionMatchParent),
		Height:          dim(core.DimensionWrapContent),
		BackgroundColor: bgColor("#FFFFFF"),
		CornerRadius:    4,
	})
	gridNode.SetPainter(&bgPainter{})
	gridColors := []string{"#E3F2FD", "#E8F5E9", "#FFF3E0", "#FCE4EC", "#F3E5F5", "#E0F7FA", "#FFF9C4", "#D7CCC8", "#CFD8DC"}
	for i := 0; i < 9; i++ {
		cell := widget.NewTextView(fmt.Sprintf("Cell %d", i+1))
		cell.Node().SetStyle(&core.Style{
			Width:           dim(core.DimensionMatchParent),
			Height:          dimVal(44),
			FontSize:        12,
			TextColor:       core.ParseColor("#333333"),
			BackgroundColor: bgColor(gridColors[i]),
			CornerRadius:    4,
		})
		cell.Node().GetStyle().Gravity = core.GravityCenter
		gridNode.AddChild(cell.Node())
	}
	page.AddChild(gridNode)

	// -- FlexLayout wrap --
	page.AddChild(sectionLabel("FlexLayout — wrap, centered"))
	flexNode := core.NewNode("FlexLayout")
	flexNode.SetLayout(&layout.FlexLayout{
		Orientation: layout.Horizontal,
		Wrap:        layout.FlexWrapOn,
		Spacing:     8,
		LineSpacing: 8,
		Justify:     layout.FlexJustifyStart,
		AlignItems:  layout.FlexAlignCenter,
	})
	flexNode.SetStyle(&core.Style{
		Width:           dim(core.DimensionMatchParent),
		Height:          dim(core.DimensionWrapContent),
		BackgroundColor: bgColor("#F5F5F5"),
		CornerRadius:    8,
	})
	flexNode.SetPainter(&bgPainter{})
	flexNode.SetPadding(core.Insets{Left: 8, Top: 8, Right: 8, Bottom: 8})

	tags := []string{"Go", "Rust", "TypeScript", "Python", "Java", "Kotlin", "Swift", "C++", "Ruby", "Dart"}
	tagColors := []string{"#1976D2", "#E64A19", "#0288D1", "#388E3C", "#D32F2F", "#7B1FA2", "#F57C00", "#455A64", "#C62828", "#00838F"}
	for i, tag := range tags {
		btn := widget.NewButton(tag, nil)
		btn.Node().SetStyle(&core.Style{
			Width:           dim(core.DimensionWrapContent),
			Height:          dimVal(30),
			BackgroundColor: bgColor(tagColors[i]),
			TextColor:       core.ParseColor("#FFFFFF"),
			CornerRadius:    15,
			FontSize:        13,
		})
		btn.SetOnClickListener(func(v core.View) {
			a.updateStatus(fmt.Sprintf("Flex: '%s'", tag))
		})
		flexNode.AddChild(btn.Node())
	}
	page.AddChild(flexNode)

	// -- FlexLayout space-between --
	page.AddChild(sectionLabel("FlexLayout — space-between"))
	flex2 := core.NewNode("FlexLayout2")
	flex2.SetLayout(&layout.FlexLayout{
		Orientation: layout.Horizontal,
		Justify:     layout.FlexJustifySpaceBetween,
		AlignItems:  layout.FlexAlignCenter,
	})
	flex2.SetStyle(&core.Style{
		Width:           dim(core.DimensionMatchParent),
		Height:          dimVal(40),
		BackgroundColor: bgColor("#ECEFF1"),
		CornerRadius:    4,
	})
	flex2.SetPainter(&bgPainter{})
	for _, label := range []string{"Start", "Middle", "End"} {
		tv := widget.NewTextView(label)
		tv.Node().SetStyle(&core.Style{
			Width:     dim(core.DimensionWrapContent),
			Height:    dim(core.DimensionWrapContent),
			FontSize:  13,
			TextColor: core.ParseColor("#37474F"),
		})
		flex2.AddChild(tv.Node())
	}
	page.AddChild(flex2)

	// -- SplitPane --
	page.AddChild(sectionLabel("SplitPane — draggable divider"))
	split := widget.NewSplitPane(layout.Horizontal, 0.5)
	split.Node().SetStyle(&core.Style{
		Width:  dim(core.DimensionMatchParent),
		Height: dimVal(120),
	})
	left := widget.NewTextView("Left Pane\n( drag divider → )")
	left.Node().SetStyle(&core.Style{
		Width:           dimWeight(1),
		Height:          dim(core.DimensionMatchParent),
		BackgroundColor: bgColor("#E3F2FD"),
		FontSize:        13,
		TextColor:       core.ParseColor("#1565C0"),
	})
	left.Node().GetStyle().Gravity = core.GravityCenter
	left.Node().GetStyle().Weight = 1
	right := widget.NewTextView("Right Pane\n( ← drag divider )")
	right.Node().SetStyle(&core.Style{
		Width:           dimWeight(1),
		Height:          dim(core.DimensionMatchParent),
		BackgroundColor: bgColor("#E8F5E9"),
		FontSize:        13,
		TextColor:       core.ParseColor("#2E7D32"),
	})
	right.Node().GetStyle().Gravity = core.GravityCenter
	right.Node().GetStyle().Weight = 1
	split.SetFirstPane(left)
	split.SetSecondPane(right)
	page.AddChild(split.Node())

	return scroll
}

// ============================================================
// Tab 4: Dialog — AlertDialog, PopupMenu, Toast, Snackbar
// ============================================================

func (a *showcasePagerAdapter) buildDialogPage() core.View {
	scroll := widget.NewScrollView()
	scroll.Node().SetStyle(&core.Style{
		Width:  dim(core.DimensionMatchParent),
		Height: dim(core.DimensionMatchParent),
	})

	page := newVPage()
	scroll.Node().AddChild(page)

	// -- AlertDialog --
	page.AddChild(sectionLabel("AlertDialog"))
	btnDialog := widget.NewButton("Show AlertDialog", nil)
	btnDialog.Node().SetStyle(&core.Style{
		Width:           dim(core.DimensionMatchParent),
		Height:          dimVal(44),
		BackgroundColor: bgColor("#1976D2"),
		TextColor:       core.ParseColor("#FFFFFF"),
		CornerRadius:    4,
		FontSize:        14,
	})
	btnDialog.SetOnClickListener(func(_ core.View) {
		widget.NewAlertDialogBuilder().
			SetTitle("Confirm Action").
			SetMessage("This is an AlertDialog with three buttons: OK, Cancel, and Help.").
			SetPositiveButton("OK", func() { a.updateStatus("Dialog: OK") }).
			SetNegativeButton("Cancel", func() { a.updateStatus("Dialog: Cancel") }).
			SetNeutralButton("Help", func() { a.updateStatus("Dialog: Help") }).
			SetOnDismissListener(func() { a.window.Invalidate() }).
			Show(page)
		a.window.Invalidate()
	})
	page.AddChild(btnDialog.Node())

	// -- PopupMenu --
	page.AddChild(sectionLabel("PopupMenu"))
	btnMenu := widget.NewButton("Show PopupMenu", nil)
	btnMenu.Node().SetStyle(&core.Style{
		Width:           dim(core.DimensionMatchParent),
		Height:          dimVal(44),
		BackgroundColor: bgColor("#388E3C"),
		TextColor:       core.ParseColor("#FFFFFF"),
		CornerRadius:    4,
		FontSize:        14,
	})
	btnMenu.SetOnClickListener(func(_ core.View) {
		menu := widget.NewMenu()
		menu.AddItem("cut", "Cut", func() { a.updateStatus("Menu: Cut") })
		menu.AddItem("copy", "Copy", func() { a.updateStatus("Menu: Copy") })
		menu.AddItem("paste", "Paste", func() { a.updateStatus("Menu: Paste") })
		menu.AddItem("selectall", "Select All", func() { a.updateStatus("Menu: Select All") })
		pm := widget.NewPopupMenu(menu)
		pm.SetOnDismissListener(func() { a.window.Invalidate() })
		b := btnMenu.Node().Bounds()
		pos := btnMenu.Node().AbsolutePosition()
		pm.ShowAtPosition(btnMenu.Node(), pos.X, pos.Y+b.Height)
		a.window.Invalidate()
	})
	page.AddChild(btnMenu.Node())

	// -- Toast --
	page.AddChild(sectionLabel("Toast"))
	btnToast := widget.NewButton("Show Toast", nil)
	btnToast.Node().SetStyle(&core.Style{
		Width:           dim(core.DimensionMatchParent),
		Height:          dimVal(44),
		BackgroundColor: bgColor("#FF9800"),
		TextColor:       core.ParseColor("#FFFFFF"),
		CornerRadius:    4,
		FontSize:        14,
	})
	btnToast.SetOnClickListener(func(_ core.View) {
		widget.ShowToast(page, "This is a Toast message!", widget.ToastShort)
		a.window.Invalidate()
	})
	page.AddChild(btnToast.Node())

	// -- Snackbar --
	page.AddChild(sectionLabel("Snackbar"))
	btnSnack := widget.NewButton("Show Snackbar", nil)
	btnSnack.Node().SetStyle(&core.Style{
		Width:           dim(core.DimensionMatchParent),
		Height:          dimVal(44),
		BackgroundColor: bgColor("#795548"),
		TextColor:       core.ParseColor("#FFFFFF"),
		CornerRadius:    4,
		FontSize:        14,
	})
	btnSnack.SetOnClickListener(func(_ core.View) {
		widget.NewSnackbar(page, "Item deleted", "UNDO", func() {
			a.updateStatus("Snackbar: Undo!")
			a.window.Invalidate()
		})
		a.window.Invalidate()
	})
	page.AddChild(btnSnack.Node())

	// -- Toolbar popup (info) --
	page.AddChild(sectionLabel("Toolbar Actions"))
	page.AddChild(bodyText("The toolbar at the top has an 'Info' action button that shows a Toast. The toolbar also supports navigation click (try the back arrow area)."))

	return scroll
}

// ============================================================
// RecyclerView adapter
// ============================================================

type listAdapter struct{ count int }

func (a *listAdapter) GetItemCount() int           { return a.count }
func (a *listAdapter) GetItemViewType(pos int) int { return 0 }

func (a *listAdapter) CreateViewHolder(viewType int) *widget.ViewHolder {
	tv := widget.NewTextView("")
	tv.Node().SetStyle(&core.Style{
		FontSize:  14,
		TextColor: core.ParseColor("#212121"),
	})
	tv.Node().SetPadding(core.Insets{Left: 16, Top: 10, Right: 16, Bottom: 10})
	return &widget.ViewHolder{ItemView: tv}
}

func (a *listAdapter) BindViewHolder(holder *widget.ViewHolder, position int) {
	if tv, ok := holder.ItemView.(*widget.TextView); ok {
		tv.SetText(fmt.Sprintf("Item #%d — RecyclerView with view recycling", position+1))
	}
}

// ============================================================
// Native EditText attachment (after first render for DPI)
// ============================================================

func attachNativeEdits(root *core.Node, application *app.Application, window platform.Window) {
	dpiScale := window.GetDPI() / 96.0
	attach := func(id, placeholder string, inputType platform.InputType) {
		if v := root.FindViewById(id); v != nil {
			ne := application.Platform().CreateNativeEditText(window)
			if ne != nil {
				ne.AttachToNode(v.Node())
				ne.SetFont("Segoe UI", 14*dpiScale, 400)
				ne.SetPlaceholder(placeholder)
				ne.SetInputType(inputType)
			}
		}
	}
	attach("et_name", "Enter your name", platform.InputTypeText)
	attach("et_password", "Password", platform.InputTypePassword)
}

// ============================================================
// Helpers
// ============================================================

var tabNames = []string{"Basic", "Input", "Lists", "Layout", "Dialog"}

func newVNode(id string, w, h core.DimensionUnit) *core.Node {
	n := core.NewNode(id)
	n.SetLayout(&layout.LinearLayout{Orientation: layout.Vertical})
	n.SetStyle(&core.Style{Width: dim(w), Height: dim(h)})
	n.SetPainter(&bgPainter{})
	return n
}

func newHNode(id string, spacing float64) *core.Node {
	n := core.NewNode(id)
	n.SetLayout(&layout.LinearLayout{Orientation: layout.Horizontal, Spacing: spacing})
	n.SetStyle(&core.Style{
		Width:  dim(core.DimensionMatchParent),
		Height: dim(core.DimensionWrapContent),
	})
	n.SetPainter(&bgPainter{})
	return n
}

func newVPage() *core.Node {
	n := core.NewNode("Page")
	n.SetLayout(&layout.LinearLayout{Orientation: layout.Vertical, Spacing: 10})
	n.SetStyle(&core.Style{
		Width:           dim(core.DimensionMatchParent),
		Height:          dim(core.DimensionWrapContent),
		BackgroundColor: bgColor("#FFFFFF"),
	})
	n.SetPainter(&bgPainter{})
	n.SetPadding(core.Insets{Left: 16, Top: 12, Right: 16, Bottom: 16})
	return n
}

func sectionLabel(text string) *core.Node {
	tv := widget.NewTextView(text)
	tv.Node().SetStyle(&core.Style{
		Width:     dim(core.DimensionMatchParent),
		Height:    dim(core.DimensionWrapContent),
		FontSize:  13,
		TextColor: core.ParseColor("#757575"),
	})
	return tv.Node()
}

func bodyText(text string) *core.Node {
	tv := widget.NewTextView(text)
	tv.Node().SetStyle(&core.Style{
		Width:     dim(core.DimensionMatchParent),
		Height:    dim(core.DimensionWrapContent),
		FontSize:  14,
		TextColor: core.ParseColor("#333333"),
	})
	return tv.Node()
}

func dim(t core.DimensionUnit) core.Dimension {
	return core.Dimension{Unit: t}
}

func dimVal(v float64) core.Dimension {
	return core.Dimension{Value: v, Unit: core.DimensionDp}
}

func dimWeight(w float64) core.Dimension {
	return core.Dimension{Unit: core.DimensionWeight, Value: w}
}

func bgColor(hex string) color.RGBA {
	return core.ParseColor(hex)
}

// bgPainter renders background/border for container nodes with a Layout.
type bgPainter struct{}

func (p *bgPainter) Measure(_ *core.Node, ws, hs core.MeasureSpec) core.Size {
	w, h := 0.0, 0.0
	if ws.Mode == core.MeasureModeExact {
		w = ws.Size
	}
	if hs.Mode == core.MeasureModeExact {
		h = hs.Size
	}
	return core.Size{Width: w, Height: h}
}

func (p *bgPainter) Paint(node *core.Node, canvas core.Canvas) {
	s := node.GetStyle()
	if s == nil {
		return
	}
	b := node.Bounds()
	if s.BackgroundColor.A > 0 {
		paint := &core.Paint{Color: s.BackgroundColor, DrawStyle: core.PaintFill}
		if s.CornerRadius > 0 {
			canvas.DrawRoundRect(core.Rect{Width: b.Width, Height: b.Height}, s.CornerRadius, paint)
		} else {
			canvas.DrawRect(core.Rect{Width: b.Width, Height: b.Height}, paint)
		}
	}
	if s.BorderWidth > 0 && s.BorderColor.A > 0 {
		paint := &core.Paint{Color: s.BorderColor, DrawStyle: core.PaintStroke, StrokeWidth: s.BorderWidth}
		if s.CornerRadius > 0 {
			canvas.DrawRoundRect(core.Rect{Width: b.Width, Height: b.Height}, s.CornerRadius, paint)
		} else {
			canvas.DrawRect(core.Rect{Width: b.Width, Height: b.Height}, paint)
		}
	}
}

// ============================================================
// Utilities
// ============================================================

func hasFlag(flag string) bool {
	for _, arg := range os.Args[1:] {
		if arg == flag {
			return true
		}
	}
	return false
}

func outputDir() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "output")
}

func savePNG(path string, img *image.RGBA) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}
