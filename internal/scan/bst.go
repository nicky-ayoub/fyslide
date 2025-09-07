package scan

// BSTNode represents a node in the Binary Search Tree.
type BSTNode struct {
	Item  FileItem
	Left  *BSTNode
	Right *BSTNode
}

// FileItemBST represents the Binary Search Tree for FileItems.
type FileItemBST struct {
	Root *BSTNode
}

// NewFileItemBST creates a new, empty BST.
func NewFileItemBST() *FileItemBST {
	return &FileItemBST{}
}

// Insert adds a new FileItem to the BST, maintaining sorted order by path.
// It recursively finds the correct position for the new item.
func (t *FileItemBST) Insert(item FileItem) {
	t.Root = t.insert(t.Root, item)
}

// insert is a recursive helper function for Insert.
func (t *FileItemBST) insert(node *BSTNode, item FileItem) *BSTNode {
	// If the current node is nil, we've found the insertion point.
	if node == nil {
		return &BSTNode{Item: item}
	}

	// Compare paths to decide whether to go left or right.
	if item.Path < node.Item.Path {
		node.Left = t.insert(node.Left, item)
	} else if item.Path > node.Item.Path {
		// We only insert if the path is not a duplicate.
		node.Right = t.insert(node.Right, item)
	}

	return node
}

// ToSlice performs an in-order traversal of the tree to return all items
// as a perfectly sorted slice of FileItems.
func (t *FileItemBST) ToSlice() FileItems {
	var items FileItems
	t.inOrder(t.Root, &items)
	return items
}

// inOrder is the recursive helper for the in-order traversal.
func (t *FileItemBST) inOrder(node *BSTNode, items *FileItems) {
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
