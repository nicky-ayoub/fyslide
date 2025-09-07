package scan

// AugmentedBSTNode represents a node in an Order-Statistic Tree (a type of Augmented BST).
// It includes a 'Size' field to enable efficient index-based lookups.
type AugmentedBSTNode struct {
	Item   FileItem
	Left   *AugmentedBSTNode
	Right  *AugmentedBSTNode
	Size   int // The number of nodes in the subtree rooted at this node (including itself)
	Height int // The height of the subtree rooted at this node
}

// FileItemAugmentedBST represents the Augmented Binary Search Tree for FileItems.
type FileItemAugmentedBST struct {
	Root *AugmentedBSTNode
}

// NewFileItemAugmentedBST creates a new, empty Augmented BST.
func NewFileItemAugmentedBST() *FileItemAugmentedBST {
	return &FileItemAugmentedBST{}
}

// size is a helper function to safely get the size of a node's subtree.
func size(n *AugmentedBSTNode) int {
	if n == nil {
		return 0
	}
	return n.Size
}

// height is a helper function to safely get the height of a node's subtree.
func height(n *AugmentedBSTNode) int {
	if n == nil {
		return 0
	}
	return n.Height
}

// bstMax returns the greater of two integers.
func bstMax(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// updateSize recalculates the size of a node based on its children.
func (n *AugmentedBSTNode) updateSize() {
	n.Size = 1 + size(n.Left) + size(n.Right)
}

func (n *AugmentedBSTNode) updateHeight() {
	n.Height = 1 + bstMax(height(n.Left), height(n.Right))
}

// Insert adds a new FileItem to the BST, maintaining sorted order by path.
// It recursively finds the correct position for the new item.
func (t *FileItemAugmentedBST) Insert(item FileItem) {
	t.Root = t.insert(t.Root, item)
}

// getBalance calculates the balance factor of a node.
func (t *FileItemAugmentedBST) getBalance(n *AugmentedBSTNode) int {
	if n == nil {
		return 0
	}
	return height(n.Left) - height(n.Right)
}

// insert is a recursive helper function for Insert.
func (t *FileItemAugmentedBST) insert(node *AugmentedBSTNode, item FileItem) *AugmentedBSTNode {
	// If the current node is nil, we've found the insertion point.
	if node == nil {
		return &AugmentedBSTNode{Item: item, Size: 1, Height: 1}
	}

	// Compare paths to decide whether to go left or right.
	if item.Path < node.Item.Path {
		node.Left = t.insert(node.Left, item)
	} else if item.Path > node.Item.Path {
		// We only insert if the path is not a duplicate.
		node.Right = t.insert(node.Right, item)
	} else {
		return node // Duplicate path, do not insert.
	}

	// After insertion in a subtree, update the size and height of the current node.
	node.updateSize()
	node.updateHeight()

	// Get the balance factor and rebalance if necessary.
	balance := t.getBalance(node)

	// Left Left Case
	if balance > 1 && item.Path < node.Left.Item.Path {
		return t.rightRotate(node)
	}
	// Right Right Case
	if balance < -1 && item.Path > node.Right.Item.Path {
		return t.leftRotate(node)
	}
	// Left Right Case
	if balance > 1 && item.Path > node.Left.Item.Path {
		node.Left = t.leftRotate(node.Left)
		return t.rightRotate(node)
	}
	// Right Left Case
	if balance < -1 && item.Path < node.Right.Item.Path {
		node.Right = t.rightRotate(node.Right)
		return t.leftRotate(node)
	}

	return node
}

// rightRotate performs a right rotation on the subtree rooted at y.
// It is a fundamental operation for self-balancing trees like AVL or Red-Black trees.
func (t *FileItemAugmentedBST) rightRotate(y *AugmentedBSTNode) *AugmentedBSTNode {
	x := y.Left
	T2 := x.Right

	// Perform rotation
	x.Right = y
	y.Left = T2

	// Update sizes. Order is important: y's size must be updated before x's.
	y.updateSize()
	y.updateHeight()
	x.updateSize()
	x.updateHeight()

	// Return new root
	return x
}

// leftRotate performs a left rotation on the subtree rooted at x.
func (t *FileItemAugmentedBST) leftRotate(x *AugmentedBSTNode) *AugmentedBSTNode {
	y := x.Right
	T2 := y.Left

	y.Left = x
	x.Right = T2

	x.updateSize()
	x.updateHeight()
	y.updateSize()
	y.updateHeight()

	return y
}

// Remove deletes an item from the BST based on its path.
func (t *FileItemAugmentedBST) Remove(path string) {
	t.Root = t.remove(t.Root, path)
}

// remove is the recursive helper for deletion.
func (t *FileItemAugmentedBST) remove(node *AugmentedBSTNode, path string) *AugmentedBSTNode {
	if node == nil {
		return nil // Item not found, nothing to do.
	}

	// Find the node to remove.
	if path < node.Item.Path {
		node.Left = t.remove(node.Left, path)
	} else if path > node.Item.Path {
		node.Right = t.remove(node.Right, path)
	} else {
		// Node found. Handle the three cases for deletion.
		// Case 1: Node has no left child.
		if node.Left == nil {
			return node.Right
		}
		// Case 2: Node has no right child.
		if node.Right == nil {
			return node.Left
		}

		// Case 3: Node has two children.
		// Find the in-order successor (smallest node in the right subtree).
		temp := t.findMin(node.Right)
		// Copy the successor's data to this node.
		node.Item = temp.Item
		// Delete the in-order successor from the right subtree.
		node.Right = t.remove(node.Right, temp.Item.Path)
	}

	node.updateSize()
	node.updateHeight()

	// Get the balance factor and rebalance if necessary.
	balance := t.getBalance(node)

	// Left Left Case
	if balance > 1 && t.getBalance(node.Left) >= 0 {
		return t.rightRotate(node)
	}
	// Left Right Case
	if balance > 1 && t.getBalance(node.Left) < 0 {
		node.Left = t.leftRotate(node.Left)
		return t.rightRotate(node)
	}
	// Right Right Case
	if balance < -1 && t.getBalance(node.Right) <= 0 {
		return t.leftRotate(node)
	}
	// Right Left Case
	if balance < -1 && t.getBalance(node.Right) > 0 {
		node.Right = t.rightRotate(node.Right)
		return t.leftRotate(node)
	}

	return node
}

// findMin finds the node with the minimum value (path) in a given subtree.
func (t *FileItemAugmentedBST) findMin(node *AugmentedBSTNode) *AugmentedBSTNode {
	for node != nil && node.Left != nil {
		node = node.Left
	}
	return node
}

// GetItemByIndex finds the k-th smallest item in the tree (0-indexed).
// This is the core advantage of an augmented tree, providing O(log n) access.
func (t *FileItemAugmentedBST) GetItemByIndex(index int) (*FileItem, bool) {
	if t.Root == nil || index < 0 || index >= t.Root.Size {
		return nil, false
	}
	return t.selectNode(t.Root, index), true
}

// selectNode is the recursive helper to find the node at a specific index.
func (t *FileItemAugmentedBST) selectNode(node *AugmentedBSTNode, index int) *FileItem {
	if node == nil {
		return nil // Should not happen if bounds are checked
	}

	// The rank of the current node is the number of items in its left subtree.
	leftSize := size(node.Left)

	if index < leftSize {
		// The target is in the left subtree.
		return t.selectNode(node.Left, index)
	} else if index > leftSize {
		// The target is in the right subtree. We adjust the index accordingly.
		return t.selectNode(node.Right, index-leftSize-1)
	}

	// If index == leftSize, we've found our node.
	return &node.Item
}

// GetIndexOfItem finds the 0-based index of an item with a given path.
// Returns the index and true if found, or -1 and false if not found.
func (t *FileItemAugmentedBST) GetIndexOfItem(path string) (int, bool) {
	return t.rank(t.Root, path)
}

// rank is the recursive helper to find the index of a given path.
func (t *FileItemAugmentedBST) rank(node *AugmentedBSTNode, path string) (int, bool) {
	if node == nil {
		return -1, false // Not found
	}

	if path < node.Item.Path {
		return t.rank(node.Left, path)
	} else if path > node.Item.Path {
		// The index in the right subtree is relative, so we add the size of the
		// left subtree and the root (1) to it.
		rightRank, found := t.rank(node.Right, path)
		if !found {
			return -1, false
		}
		return size(node.Left) + 1 + rightRank, true
	}

	// Found the node. Its index is the size of its left subtree.
	return size(node.Left), true
}

// ToSlice performs an in-order traversal of the tree to return all items
// as a perfectly sorted slice of FileItems.
func (t *FileItemAugmentedBST) ToSlice() FileItems {
	var items FileItems
	if t.Root == nil {
		return items
	}
	// Pre-allocate the slice for efficiency since we know the final size.
	items = make(FileItems, 0, t.Root.Size)
	t.inOrder(t.Root, &items)
	return items
}

// inOrder is the recursive helper for the in-order traversal.
func (t *FileItemAugmentedBST) inOrder(node *AugmentedBSTNode, items *FileItems) {
	if node == nil {
		return
	}

	// 1. Traverse the left subtree.
	t.inOrder(node.Left, items)

	// 2. Visit the root node (append its item).
	*items = append(*items, node.Item)

	// 3. Traverse the right subtree.
	t.inOrder(node.Right, items)
}
