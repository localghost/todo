// "Show" next to a password field switches it between hidden and readable.
document.addEventListener("click", (e) => {
  const button = e.target.closest && e.target.closest(".show-password");
  if (!button) {
    return;
  }
  const input = document.getElementById(button.dataset.target);
  const show = input.type === "password";
  input.type = show ? "text" : "password";
  button.textContent = show ? "Hide" : "Show";
  button.setAttribute("aria-pressed", show ? "true" : "false");
});
