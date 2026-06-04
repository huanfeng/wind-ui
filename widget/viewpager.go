package widget

import (
	"github.com/huanfeng/wind-ui/core"
)

// PagerAdapter provides pages to a ViewPager.
type PagerAdapter interface {
	// GetCount returns the total number of pages.
	GetCount() int
	// CreatePage creates the view tree for the given page index.
	CreatePage(index int) core.View
	// GetPageTitle returns an optional title for the page (used by TabLayout).
	GetPageTitle(index int) string
}

// ViewPager is a container that displays one page at a time and supports
// programmatic page switching. It can be linked with a TabLayout.
// Modeled after Android's ViewPager.
type ViewPager struct {
	BaseView
	adapter       PagerAdapter
	currentPage   int
	pages         []core.View // cached page views
	onPageChanged func(index int)
	tabLayout     *TabLayout // linked TabLayout
}

// NewViewPager creates a new ViewPager.
func NewViewPager() *ViewPager {
	vp := &ViewPager{
		currentPage: 0,
	}
	vp.node = initNode("ViewPager", vp)
	vp.node.SetPainter(&viewPagerPainter{vp: vp})
	vp.node.SetStyle(&core.Style{})
	vp.node.SetData("paintsChildren", true)
	return vp
}

// SetAdapter sets the pager adapter and rebuilds all pages.
func (vp *ViewPager) SetAdapter(adapter PagerAdapter) {
	vp.adapter = adapter
	vp.rebuildPages()
	// Sync linked TabLayout
	if vp.tabLayout != nil && adapter != nil {
		vp.syncTabLayout()
	}
}

// GetAdapter returns the current adapter.
func (vp *ViewPager) GetAdapter() PagerAdapter {
	return vp.adapter
}

// SetCurrentPage switches to the given page index.
func (vp *ViewPager) SetCurrentPage(index int) {
	if vp.adapter == nil || index < 0 || index >= vp.adapter.GetCount() {
		return
	}
	if vp.currentPage == index {
		return
	}
	vp.currentPage = index
	vp.updateVisibility()
	vp.node.MarkDirty()
	if vp.onPageChanged != nil {
		vp.onPageChanged(index)
	}
	// Sync linked TabLayout
	if vp.tabLayout != nil {
		vp.tabLayout.selectedTab = index
		vp.tabLayout.Node().MarkDirty()
	}
}

// GetCurrentPage returns the current page index.
func (vp *ViewPager) GetCurrentPage() int {
	return vp.currentPage
}

// GetPageCount returns the number of pages.
func (vp *ViewPager) GetPageCount() int {
	if vp.adapter == nil {
		return 0
	}
	return vp.adapter.GetCount()
}

// SetOnPageChangedListener sets the callback invoked when the page changes.
func (vp *ViewPager) SetOnPageChangedListener(fn func(index int)) {
	vp.onPageChanged = fn
}

// SetupWithTabLayout links this ViewPager with a TabLayout for synchronized
// tab/page switching.
func (vp *ViewPager) SetupWithTabLayout(tl *TabLayout) {
	vp.tabLayout = tl
	if vp.adapter != nil {
		vp.syncTabLayout()
	}
	tl.SetOnTabSelectedListener(func(index int) {
		vp.SetCurrentPage(index)
	})
}

// syncTabLayout populates the linked TabLayout with page titles from the adapter.
func (vp *ViewPager) syncTabLayout() {
	if vp.tabLayout == nil || vp.adapter == nil {
		return
	}
	// Clear existing tabs
	vp.tabLayout.tabs = nil
	for i := 0; i < vp.adapter.GetCount(); i++ {
		title := vp.adapter.GetPageTitle(i)
		if title == "" {
			title = "Page"
		}
		vp.tabLayout.AddTab(Tab{Text: title})
	}
	vp.tabLayout.selectedTab = vp.currentPage
	vp.tabLayout.Node().MarkDirty()
}

// rebuildPages creates all page views from the adapter and adds them as children.
func (vp *ViewPager) rebuildPages() {
	// Remove old children
	for _, child := range vp.node.Children() {
		vp.node.RemoveChild(child)
	}
	vp.pages = nil

	if vp.adapter == nil {
		return
	}

	count := vp.adapter.GetCount()
	vp.pages = make([]core.View, count)
	for i := 0; i < count; i++ {
		page := vp.adapter.CreatePage(i)
		vp.pages[i] = page
		vp.node.AddChild(page.Node())
	}

	if vp.currentPage >= count {
		vp.currentPage = 0
	}
	vp.updateVisibility()
}

// updateVisibility shows only the current page and hides all others.
// Native edit controls on hidden pages are explicitly hidden so they don't
// remain visible on screen after a tab switch.
func (vp *ViewPager) updateVisibility() {
	for i, page := range vp.pages {
		if i == vp.currentPage {
			page.Node().SetVisibility(core.Visible)
		} else {
			page.Node().SetVisibility(core.Gone)
			hideNativeEdits(page.Node())
		}
	}
}

// nativeEditHide is implemented by platform native edit controls that can be
// explicitly hidden when their parent page becomes invisible.
type nativeEditHide interface {
	Hide()
}

// hideNativeEdits walks the node tree and hides any attached native edit controls.
func hideNativeEdits(n *core.Node) {
	if ne, ok := n.GetData("nativeEdit").(nativeEditHide); ok {
		ne.Hide()
	}
	for _, child := range n.Children() {
		hideNativeEdits(child)
	}
}

// ---------- viewPagerPainter ----------

type viewPagerPainter struct {
	vp *ViewPager
}

func (p *viewPagerPainter) Measure(node *core.Node, ws, hs core.MeasureSpec) core.Size {
	w, h := 0.0, 0.0
	if ws.Mode == core.MeasureModeExact {
		w = ws.Size
	}
	if hs.Mode == core.MeasureModeExact {
		h = hs.Size
	}

	// Measure the current visible page
	vp := p.vp
	if vp.currentPage >= 0 && vp.currentPage < len(vp.pages) {
		page := vp.pages[vp.currentPage]
		childWS := ws
		childHS := hs
		childSize := core.Size{}
		if painter := page.Node().GetPainter(); painter != nil {
			childSize = painter.Measure(page.Node(), childWS, childHS)
		} else if layout := page.Node().GetLayout(); layout != nil {
			childSize = layout.Measure(page.Node(), childWS, childHS)
		}
		page.Node().SetMeasuredSize(childSize)
		if ws.Mode != core.MeasureModeExact && childSize.Width > w {
			w = childSize.Width
		}
		if hs.Mode != core.MeasureModeExact && childSize.Height > h {
			h = childSize.Height
		}
	}

	return core.Size{Width: w, Height: h}
}

func (p *viewPagerPainter) Paint(node *core.Node, canvas core.Canvas) {
	s := node.GetStyle()
	b := node.Bounds()
	localRect := core.Rect{Width: b.Width, Height: b.Height}

	// Background
	if s != nil && s.BackgroundColor.A > 0 {
		bgPaint := &core.Paint{Color: s.BackgroundColor, DrawStyle: core.PaintFill}
		canvas.DrawRect(localRect, bgPaint)
	}

	// Paint the current visible page
	vp := p.vp
	if vp.currentPage >= 0 && vp.currentPage < len(vp.pages) {
		page := vp.pages[vp.currentPage]
		if page.Node().GetVisibility() == core.Visible {
			pageNode := page.Node()
			// Set page bounds to fill pager
			pageNode.SetBounds(core.Rect{X: 0, Y: 0, Width: b.Width, Height: b.Height})
			pageNode.SetMeasuredSize(core.Size{Width: b.Width, Height: b.Height})

			// Measure and arrange the page's children so they get proper bounds
			if l := pageNode.GetLayout(); l != nil {
				ws := core.MeasureSpec{Mode: core.MeasureModeExact, Size: b.Width}
				hs := core.MeasureSpec{Mode: core.MeasureModeExact, Size: b.Height}
				l.Measure(pageNode, ws, hs)
				l.Arrange(pageNode, pageNode.Bounds())
			}

			core.PaintNodeRecursive(pageNode, canvas)
		}
	}
}
