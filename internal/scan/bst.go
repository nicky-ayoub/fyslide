package scan

// AugmentedBSTNode represents a node in an Order-Statistic Tree (a type of Augmented BST).
// It includes a 'Size' field to enable efficient index-based lookups.
type AugmentedBSTNode struct {
	Item  FileItem
	Left  *AugmentedBSTNode
	Right *AugmentedBSTNode
	Size  int // The number of nodes in the subtree rooted at this node (including itself)
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

// updateSize recalculates the size of a node based on its children.
func (n *AugmentedBSTNode) updateSize() {
	n.Size = 1 + size(n.Left) + size(n.Right)
}

// Insert adds a new FileItem to the BST, maintaining sorted order by path.
// It recursively finds the correct position for the new item.
func (t *FileItemAugmentedBST) Insert(item FileItem) {
	t.Root = t.insert(t.Root, item)
}

// insert is a recursive helper function for Insert.
func (t *FileItemAugmentedBST) insert(node *AugmentedBSTNode, item FileItem) *AugmentedBSTNode {
	// If the current node is nil, we've found the insertion point.
	if node == nil {
		return &AugmentedBSTNode{Item: item, Size: 1}
	}

	// Compare paths to decide whether to go left or right.
	if item.Path < node.Item.Path {
		node.Left = t.insert(node.Left, item)
	} else if item.Path > node.Item.Path {
		// We only insert if the path is not a duplicate.
		node.Right = t.insert(node.Right, item)
	}

	// After insertion in a subtree, update the size of the current node.
	node.updateSize()
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
	x.updateSize()

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
	y.updateSize()

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
