let isSelectionMode = false;

// Check for error messages in URL
const urlParams = new URLSearchParams(window.location.search);
if (urlParams.get("error") === "empty") {
  alert("Can't upload empty file");
  // Clean up URL
  const newUrl = window.location.pathname + window.location.hash;
  window.history.replaceState({}, document.title, newUrl);
}
// Note: We need to wait for DOM or place script at end. Using getElementById immediately requires elements to exist.
const actionBox = document.getElementById("action-bar");
const countLabel = document.getElementById("selected-count");
const selectBtn = document.getElementById("select-toggle-btn");

// File upload validation
const fileInput = document.getElementById("file-upload-input");
const pushBtn = document.getElementById("push-server-btn");
const uploadForm = document.querySelector(".upload-section form");

if (fileInput && pushBtn) {
  fileInput.addEventListener("change", () => {
    if (fileInput.files && fileInput.files.length > 0) {
      pushBtn.disabled = false;
      pushBtn.style.opacity = "1";
      pushBtn.style.cursor = "pointer";
    } else {
      pushBtn.disabled = true;
      pushBtn.style.opacity = "0.5";
      pushBtn.style.cursor = "not-allowed";
    }
  });
}

// Progress bar elements
const progressOverlay = document.getElementById("progress-overlay");
const progressLabel = document.getElementById("progress-label");
const progressFill = document.getElementById("progress-bar-fill");
const progressPercent = document.getElementById("progress-percent");

function showProgress(label, percent) {
  progressOverlay.classList.add("active");
  progressLabel.textContent = label;
  progressFill.style.width = percent + "%";
  progressPercent.textContent = Math.round(percent) + "%";
}

function hideProgress() {
  progressOverlay.classList.remove("active");
  progressFill.style.width = "0%";
  progressPercent.textContent = "0%";
}

// AJAX upload with progress tracking
if (uploadForm) {
  uploadForm.addEventListener("submit", (e) => {
    e.preventDefault();
    const formData = new FormData(uploadForm);
    pushBtn.disabled = true;

    const xhr = new XMLHttpRequest();
    xhr.open("POST", uploadForm.action, true);

    xhr.upload.onprogress = (event) => {
      if (event.lengthComputable) {
        const percent = (event.loaded / event.total) * 100;
        showProgress("UPLOADING...", percent);
      }
    };

    xhr.onload = () => {
      hideProgress();
      if (xhr.status >= 200 && xhr.status < 400) {
        window.location.reload();
      } else {
        alert("Upload failed");
        pushBtn.disabled = false;
      }
    };

    xhr.onerror = () => {
      hideProgress();
      alert("Upload error");
      pushBtn.disabled = false;
    };

    xhr.send(formData);
  });
}

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
  if (selected.length === 0) return;

  let url = "/zip?";
  selected.forEach((item) => {
    const path = item.getAttribute("data-path");
    url += "p=" + encodeURIComponent(path) + "&";
  });

  const xhr = new XMLHttpRequest();
  xhr.open("GET", url, true);
  xhr.responseType = "blob";

  xhr.onprogress = (event) => {
    if (event.lengthComputable) {
      const percent = (event.loaded / event.total) * 100;
      showProgress("DOWNLOADING...", percent);
    } else {
      // Server doesn't send Content-Length, show indeterminate
      showProgress("DOWNLOADING...", 50);
    }
  };

  xhr.onload = () => {
    hideProgress();
    if (xhr.status === 200) {
      const blob = xhr.response;
      const link = document.createElement("a");
      link.href = URL.createObjectURL(blob);
      link.download = "invifiles_bundle.zip";
      link.click();
      URL.revokeObjectURL(link.href);
      clearSelection();
    } else {
      alert("Download failed");
    }
  };

  xhr.onerror = () => {
    hideProgress();
    alert("Download error");
  };

  showProgress("DOWNLOADING...", 0);
  xhr.send();
}
