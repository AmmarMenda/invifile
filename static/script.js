let isSelectionMode = false;
// Note: We need to wait for DOM or place script at end. Using getElementById immediately requires elements to exist.
const actionBox = document.getElementById("action-bar");
const countLabel = document.getElementById("selected-count");
const selectBtn = document.getElementById("select-toggle-btn");

// Add listeners to all items
document.querySelectorAll(".item").forEach((item) => {
  item.addEventListener("click", (e) => {
    if (isSelectionMode) {
      e.preventDefault(); // Prevent navigation
      item.classList.toggle("selected");
      updateActionBar();
    } else {
      // Normal behavior
    }
  });
});

function toggleSelectionMode() {
  isSelectionMode = !isSelectionMode;
  if (isSelectionMode) {
    selectBtn.classList.add("active");
    selectBtn.innerText = "CANCEL SELECTION";
  } else {
    clearSelection();
    selectBtn.classList.remove("active");
    selectBtn.innerText = "SELECT ITEMS";
  }
}

function startSelection(firstItem) {
  if (!isSelectionMode) toggleSelectionMode();
  firstItem.classList.add("selected");
  updateActionBar();
}

function updateActionBar() {
  const selected = document.querySelectorAll(".item.selected");
  countLabel.innerText = selected.length + " SELECTED";

  if (selected.length > 0) {
    actionBox.classList.add("active");
  } else {
    actionBox.classList.remove("active");
  }
}

function clearSelection() {
  document
    .querySelectorAll(".item.selected")
    .forEach((i) => i.classList.remove("selected"));
  isSelectionMode = false;
  selectBtn.classList.remove("active");
  selectBtn.innerText = "SELECT ITEMS";
  updateActionBar();
}

// Logic for actions
function sortGrid(criteria) {
  const grid = document.getElementById("fileGrid");
  const items = Array.from(grid.getElementsByClassName("item"));

  items.sort((a, b) => {
    const aIsDir = a.getAttribute("data-type") === "dir";
    const bIsDir = b.getAttribute("data-type") === "dir";
    if (aIsDir && !bIsDir) return -1;
    if (!aIsDir && bIsDir) return 1;

    let valA = a.getAttribute("data-" + criteria);
    let valB = b.getAttribute("data-" + criteria);

    if (criteria === "size" || criteria === "date") {
      return parseFloat(valB) - parseFloat(valA);
    }

    return valA.localeCompare(valB);
  });

  items.forEach((item) => grid.appendChild(item));
}
function downloadSelected() {
  const selected = document.querySelectorAll(".item.selected");
  let url = "/zip?";

  selected.forEach((item) => {
    const path = item.getAttribute("data-path");
    url += "p=" + encodeURIComponent(path) + "&";
  });

  window.location.href = url;
}
