package core

// Visibility controls whether a node is drawn and takes up layout space.
type Visibility int

const (
	Visible   Visibility = iota // Drawn and takes space
	Invisible                   // Not drawn but takes space
	Gone                        // Not drawn and takes no space
)

// View is the public interface for all UI elements (widgets).
type View interface {
	Node() *Node
	SetId(id string)
	GetId() string
	SetVisibility(v Visibility)
	GetVisibility() Visibility
	SetEnabled(enabled bool)
	IsEnabled() bool
}

// Node is the internal tree element that backs every View.
type Node struct {
	parent   *Node
	children []*Node

	bounds  Rect
	padding Insets
	margin  Insets

	layout  Layout
	painter Painter
	handler Handler
	style   *Style

	id         string
	tag        string
	visibility Visibility
	enabled    bool
	dirty      bool
	childDirty bool

	// Layout dirty: set when structural/sizing changes require re-measure/arrange.
	// Pure visual changes (color, hover state) do NOT set this flag.
	layoutDirty bool

	// Dirty region tracking (only used on root node).
	dirtyRects []Rect
	dirtyFull  bool // true = degrade to full repaint

	// View association — the widget wrapping this node
	view View

	measuredSize Size
	data         map[string]interface{}
}

// NewNode creates a new Node with the given tag name.
func NewNode(tag string) *Node {
	return &Node{
		tag:         tag,
		visibility:  Visible,
		enabled:     true,
		dirty:       true,
		layoutDirty: true,
		data:        make(map[string]interface{}),
	}
}

// ---------- Tree operations ----------

// Parent returns the parent node, or nil if this is a root.
func (n *Node) Parent() *Node {
	return n.parent
}

// Children returns the list of child nodes.
func (n *Node) Children() []*Node {
	return n.children
}

// Tag returns the tag name of this node (e.g. "LinearLayout", "TextView").
func (n *Node) Tag() string {
	return n.tag
}

// AddChild appends a child node and sets its parent.
// If the tree has a dpiScale set, the new child subtree is automatically
// DPI-scaled to match (prevents the need for manual scaling of dynamic nodes).
func (n *Node) AddChild(child *Node) {
	if child == nil {
		return
	}
	// Remove from previous parent if any
	if child.parent != nil {
		child.parent.RemoveChild(child)
	}
	child.parent = n
	n.children = append(n.children, child)
	n.MarkDirty()
	n.MarkLayoutDirty()

	// Auto-DPI-scale: if any ancestor has dpiScale set and this child
	// hasn't been scaled yet, scale it now.
	if child.GetData("dpiScaled") == nil {
		if s, ok := n.findDPIScale(); ok && s != 0 {
			ScaleNodeDPI(child, s)
		}
	}
}

// findDPIScale walks up the parent chain to find a dpiScale value.
func (n *Node) findDPIScale() (float64, bool) {
	for node := n; node != nil; node = node.parent {
		if s, ok := node.GetData("dpiScale").(float64); ok && s > 0 {
			return s, true
		}
	}
	return 0, false
}

// RemoveChild removes a child node and clears its parent reference.
func (n *Node) RemoveChild(child *Node) {
	if child == nil {
		return
	}
	for i, c := range n.children {
		if c == child {
			n.children = append(n.children[:i], n.children[i+1:]...)
			child.parent = nil
			n.MarkDirty()
			n.MarkLayoutDirty()
			return
		}
	}
}

// ---------- ID ----------

// SetId sets the identifier for this node.
func (n *Node) SetId(id string) {
	n.id = id
}

// GetId returns the identifier of this node.
func (n *Node) GetId() string {
	return n.id
}

// Id is an alias for GetId.
func (n *Node) Id() string {
	return n.id
}

// ---------- Visibility ----------

// SetVisibility sets the visibility of this node.
func (n *Node) SetVisibility(v Visibility) {
	if n.visibility != v {
		n.visibility = v
		n.MarkDirty()
		n.MarkLayoutDirty()
	}
}

// GetVisibility returns the visibility of this node.
func (n *Node) GetVisibility() Visibility {
	return n.visibility
}

// ---------- Enabled ----------

// SetEnabled sets whether this node is enabled for interaction.
func (n *Node) SetEnabled(b bool) {
	n.enabled = b
}

// IsEnabled reports whether this node is enabled.
func (n *Node) IsEnabled() bool {
	return n.enabled
}

// ---------- Component accessors ----------

// SetLayout sets the layout strategy for this node.
func (n *Node) SetLayout(l Layout) {
	n.layout = l
}

// GetLayout returns the layout strategy, or nil.
func (n *Node) GetLayout() Layout {
	return n.layout
}

// SetPainter sets the painter for this node.
func (n *Node) SetPainter(p Painter) {
	n.painter = p
}

// GetPainter returns the painter, or nil.
func (n *Node) GetPainter() Painter {
	return n.painter
}

// SetHandler sets the event handler for this node.
func (n *Node) SetHandler(h Handler) {
	n.handler = h
}

// GetHandler returns the event handler, or nil.
func (n *Node) GetHandler() Handler {
	return n.handler
}

// SetStyle sets the style for this node.
func (n *Node) SetStyle(s *Style) {
	n.style = s
}

// GetStyle returns the style, or nil.
func (n *Node) GetStyle() *Style {
	return n.style
}

// ---------- Geometry ----------

// Bounds returns the layout bounds of this node.
func (n *Node) Bounds() Rect {
	return n.bounds
}

// SetBounds sets the layout bounds of this node.
func (n *Node) SetBounds(r Rect) {
	n.bounds = r
}

// Padding returns the padding insets.
func (n *Node) Padding() Insets {
	return n.padding
}

// SetPadding sets the padding insets.
func (n *Node) SetPadding(insets Insets) {
	n.padding = insets
}

// Margin returns the margin insets.
func (n *Node) Margin() Insets {
	return n.margin
}

// SetMargin sets the margin insets.
func (n *Node) SetMargin(insets Insets) {
	n.margin = insets
}

// MeasuredSize returns the size computed during the measure pass.
func (n *Node) MeasuredSize() Size {
	return n.measuredSize
}

// SetMeasuredSize stores the size computed during the measure pass.
func (n *Node) SetMeasuredSize(s Size) {
	n.measuredSize = s
}

// ---------- Data store ----------

// SetData stores an arbitrary value by key.
func (n *Node) SetData(key string, value interface{}) {
	if n.data == nil {
		n.data = make(map[string]interface{})
	}
	n.data[key] = value
}

// GetData retrieves a value by key, or nil if not found.
func (n *Node) GetData(key string) interface{} {
	if n.data == nil {
		return nil
	}
	return n.data[key]
}

// GetDataString retrieves a string value by key, or "" if not found or not a string.
func (n *Node) GetDataString(key string) string {
	v := n.GetData(key)
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// ---------- View association ----------

// SetView associates a View with this node.
func (n *Node) SetView(v View) {
	n.view = v
}

// GetView returns the associated View, or nil.
func (n *Node) GetView() View {
	return n.view
}

// ---------- Dirty tracking ----------

// maxDirtyRects is the upper limit of dirty rectangles tracked on the root
// node before degrading to a full repaint.
const maxDirtyRects = 8

// MarkDirty marks this node as dirty and registers its dirty region.
// This is the primary API called by widgets — it delegates to Invalidate().
func (n *Node) MarkDirty() {
	n.Invalidate()
}

// Invalidate marks the entire node as dirty and propagates its screen-space
// bounding rectangle up the parent chain to the root node (Android-style
// invalidateChild pattern).
func (n *Node) Invalidate() {
	n.dirty = true
	b := n.bounds
	n.InvalidateRect(Rect{Width: b.Width, Height: b.Height})
}

// InvalidateRect marks a local-coordinate rectangle as dirty and bubbles it
// up the parent chain, transforming coordinates at each level.
func (n *Node) InvalidateRect(localRect Rect) {
	n.dirty = true

	// Bubble childDirty + transform rect up to root.
	rect := localRect
	current := n
	for p := current.parent; p != nil; current, p = p, p.parent {
		cb := current.bounds
		rect.X += cb.X
		rect.Y += cb.Y
		if !p.childDirty {
			p.childDirty = true
		}
	}
	// current is now the root node; rect is in screen coordinates.
	current.addDirtyRect(rect)
}

// addDirtyRect adds a screen-coordinate dirty rectangle to the root node's
// dirty region, merging overlapping rectangles.
func (n *Node) addDirtyRect(rect Rect) {
	if rect.IsEmpty() {
		return
	}
	if n.dirtyFull {
		return // already full repaint
	}

	// Try to merge with an existing overlapping rect.
	for i, existing := range n.dirtyRects {
		if existing.Overlaps(rect) {
			n.dirtyRects[i] = existing.Union(rect)
			return
		}
	}

	n.dirtyRects = append(n.dirtyRects, rect)

	// Too many rects — degrade to full repaint.
	if len(n.dirtyRects) > maxDirtyRects {
		n.dirtyFull = true
		n.dirtyRects = nil
	}
}

// PopDirtyRegion returns the accumulated dirty rectangles and resets the
// dirty region state. Called by the render loop at the start of each frame.
func (n *Node) PopDirtyRegion() (rects []Rect, fullDirty bool) {
	rects = n.dirtyRects
	fullDirty = n.dirtyFull
	n.dirtyRects = nil
	n.dirtyFull = false
	return
}

// SetFullDirty forces the next frame to be a full repaint (e.g. after resize).
func (n *Node) SetFullDirty() {
	n.dirtyFull = true
	n.dirtyRects = nil
}

// IsDirty reports whether this node itself is dirty.
func (n *Node) IsDirty() bool {
	return n.dirty
}

// IsChildDirty reports whether any descendant is dirty.
func (n *Node) IsChildDirty() bool {
	return n.childDirty
}

// ClearDirty clears the dirty and childDirty flags on this node.
func (n *Node) ClearDirty() {
	n.dirty = false
	n.childDirty = false
}

// MarkLayoutDirty marks this node and its ancestors as needing re-layout.
// Call this when structural or sizing changes occur (add/remove child, text change,
// visibility change, dimension change). Do NOT call for pure visual changes.
func (n *Node) MarkLayoutDirty() {
	for node := n; node != nil; node = node.parent {
		if node.layoutDirty {
			break
		}
		node.layoutDirty = true
	}
}

// IsLayoutDirty reports whether this node or any descendant needs re-layout.
func (n *Node) IsLayoutDirty() bool {
	return n.layoutDirty
}

// ClearLayoutDirty clears the layout dirty flag on this node.
func (n *Node) ClearLayoutDirty() {
	n.layoutDirty = false
}

// ---------- DPI Scaling ----------

// originalInsets 保存 padding/margin 的原始 dp 值，用于精确重缩放。
type originalInsets struct {
	padding Insets
	margin  Insets
}

// ScaleNodeDPI 递归地将子树中的 dp 值（padding、margin、字体大小、尺寸、间距）
// 按给定的 DPI 缩放系数进行缩放。每个节点被标记以防止重复缩放。
// 首次缩放时会将原始 dp 值保存到节点 data 中，供 RescaleNodeDPI 精确重算使用。
func ScaleNodeDPI(node *Node, scale float64) {
	if node.GetData("dpiScaled") != nil {
		return // 已缩放，跳过
	}
	node.SetData("dpiScale", scale)
	node.SetData("dpiScaled", true)

	if scale == 1.0 {
		// scale=1.0 时仍需标记 dpiScale，供子节点查找；跳过实际缩放
		for _, child := range node.Children() {
			ScaleNodeDPI(child, scale)
		}
		return
	}

	// 保存原始 Style（仅首次，防止覆盖）
	if node.GetData("originalStyle") == nil {
		if s := node.GetStyle(); s != nil {
			orig := *s // 值复制，保留原始 dp 值
			node.SetData("originalStyle", &orig)
		}
	}

	// 保存原始 padding/margin（仅首次）
	if node.GetData("originalInsets") == nil {
		node.SetData("originalInsets", &originalInsets{
			padding: node.Padding(),
			margin:  node.Margin(),
		})
	}

	// 缩放 padding
	p := node.Padding()
	node.SetPadding(Insets{
		Left:   p.Left * scale,
		Top:    p.Top * scale,
		Right:  p.Right * scale,
		Bottom: p.Bottom * scale,
	})

	// 缩放 margin
	m := node.Margin()
	node.SetMargin(Insets{
		Left:   m.Left * scale,
		Top:    m.Top * scale,
		Right:  m.Right * scale,
		Bottom: m.Bottom * scale,
	})

	// 缩放 Style 中的尺寸字段
	if s := node.GetStyle(); s != nil {
		if s.FontSize > 0 {
			s.FontSize *= scale
		}
		if s.CornerRadius > 0 {
			s.CornerRadius *= scale
		}
		if s.BorderWidth > 0 {
			s.BorderWidth *= scale
		}
		if s.Width.Unit == DimensionDp && s.Width.Value > 0 {
			s.Width.Value *= scale
		}
		if s.Height.Unit == DimensionDp && s.Height.Value > 0 {
			s.Height.Value *= scale
		}
	}

	// 通过 DPIScalable 接口缩放 Layout 内部间距
	if l := node.GetLayout(); l != nil {
		if ds, ok := l.(DPIScalable); ok {
			ds.ScaleDPI(scale)
		}
	}

	// 递归处理子节点
	for _, child := range node.Children() {
		ScaleNodeDPI(child, scale)
	}
}

// RescaleNodeDPI 将整棵子树从旧 DPI 缩放比例重新缩放到新比例。
// 与 ScaleNodeDPI 不同，它从保存的原始 dp 值重新计算，避免浮点累积误差。
// 当显示器 DPI 在运行时发生变化（WM_DPICHANGED）时调用此函数。
func RescaleNodeDPI(node *Node, newScale float64) {
	// 读取当前已存储的 dpiScale
	oldScale, ok := node.GetData("dpiScale").(float64)
	if !ok || oldScale <= 0 {
		// 节点尚未被 ScaleNodeDPI 处理，使用首次缩放流程
		ScaleNodeDPI(node, newScale)
		return
	}

	if oldScale == newScale {
		// DPI 未发生变化，无需操作
		for _, child := range node.Children() {
			RescaleNodeDPI(child, newScale)
		}
		return
	}

	// 更新存储的 dpiScale
	node.SetData("dpiScale", newScale)

	// 从原始 Style 重新计算，避免浮点累积误差
	if orig, ok := node.GetData("originalStyle").(*Style); ok {
		if s := node.GetStyle(); s != nil {
			if orig.FontSize > 0 {
				s.FontSize = orig.FontSize * newScale
			}
			if orig.CornerRadius > 0 {
				s.CornerRadius = orig.CornerRadius * newScale
			}
			if orig.BorderWidth > 0 {
				s.BorderWidth = orig.BorderWidth * newScale
			}
			if orig.Width.Unit == DimensionDp && orig.Width.Value > 0 {
				s.Width.Value = orig.Width.Value * newScale
			}
			if orig.Height.Unit == DimensionDp && orig.Height.Value > 0 {
				s.Height.Value = orig.Height.Value * newScale
			}
		}
	} else if newScale != oldScale {
		// 没有原始值备份时，用 ratio 进行相对缩放（降级路径）
		ratio := newScale / oldScale
		if s := node.GetStyle(); s != nil {
			if s.FontSize > 0 {
				s.FontSize *= ratio
			}
			if s.CornerRadius > 0 {
				s.CornerRadius *= ratio
			}
			if s.BorderWidth > 0 {
				s.BorderWidth *= ratio
			}
			if s.Width.Unit == DimensionDp && s.Width.Value > 0 {
				s.Width.Value *= ratio
			}
			if s.Height.Unit == DimensionDp && s.Height.Value > 0 {
				s.Height.Value *= ratio
			}
		}
	}

	// 从原始 padding/margin 重新计算
	if origInsets, ok := node.GetData("originalInsets").(*originalInsets); ok {
		node.SetPadding(Insets{
			Left:   origInsets.padding.Left * newScale,
			Top:    origInsets.padding.Top * newScale,
			Right:  origInsets.padding.Right * newScale,
			Bottom: origInsets.padding.Bottom * newScale,
		})
		node.SetMargin(Insets{
			Left:   origInsets.margin.Left * newScale,
			Top:    origInsets.margin.Top * newScale,
			Right:  origInsets.margin.Right * newScale,
			Bottom: origInsets.margin.Bottom * newScale,
		})
	} else {
		// 没有原始值备份时，用 ratio 进行相对缩放（降级路径）
		ratio := newScale / oldScale
		p := node.Padding()
		node.SetPadding(Insets{
			Left:   p.Left * ratio,
			Top:    p.Top * ratio,
			Right:  p.Right * ratio,
			Bottom: p.Bottom * ratio,
		})
		m := node.Margin()
		node.SetMargin(Insets{
			Left:   m.Left * ratio,
			Top:    m.Top * ratio,
			Right:  m.Right * ratio,
			Bottom: m.Bottom * ratio,
		})
	}

	// 通过 DPIScalable 接口重新缩放 Layout 内部间距（使用 ratio）
	if oldScale > 0 && newScale != oldScale {
		ratio := newScale / oldScale
		if l := node.GetLayout(); l != nil {
			if ds, ok := l.(DPIScalable); ok {
				ds.ScaleDPI(ratio)
			}
		}
	}

	// 递归处理子节点
	for _, child := range node.Children() {
		RescaleNodeDPI(child, newScale)
	}
}

// ---------- Position ----------

// AbsolutePosition returns the node's position in root coordinate space
// by summing all ancestor bounds' X/Y offsets.
func (n *Node) AbsolutePosition() Point {
	x, y := 0.0, 0.0
	for node := n; node != nil; node = node.parent {
		b := node.Bounds()
		x += b.X
		y += b.Y
	}
	return Point{X: x, Y: y}
}

// ---------- Search ----------

// FindNodeById searches this node and its descendants for a node with the given id.
// Returns nil if not found.
func (n *Node) FindNodeById(id string) *Node {
	if n.id == id {
		return n
	}
	for _, child := range n.children {
		if found := child.FindNodeById(id); found != nil {
			return found
		}
	}
	return nil
}

// FindViewById searches this node and its descendants for a node with the given id
// that has an associated View. Returns nil if not found or if the node has no View.
func (n *Node) FindViewById(id string) View {
	node := n.FindNodeById(id)
	if node == nil {
		return nil
	}
	return node.view
}

// ---------- Invalidator ----------

// Invalidator is implemented by objects that can trigger a repaint
// (e.g. platform Window). Stored on the root node so widgets deep
// in the tree can request a repaint without importing the platform package.
type Invalidator interface {
	Invalidate()
}

// SetInvalidator stores an Invalidator on this node.
func (n *Node) SetInvalidator(invalidator Invalidator) {
	n.SetData("invalidator", invalidator)
}

// GetInvalidator walks up the parent chain to find the nearest Invalidator.
// Returns nil if none found.
func (n *Node) GetInvalidator() Invalidator {
	for node := n; node != nil; node = node.parent {
		if inv, ok := node.GetData("invalidator").(Invalidator); ok {
			return inv
		}
	}
	return nil
}
