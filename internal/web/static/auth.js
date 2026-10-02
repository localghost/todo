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
});

// Sign-up: before the form is sent, find a proof-of-work nonce (about one
// second). The server checks SHA-256(form_token + ":" + nonce).
const signupForm = document.getElementById("signup-form");
if (signupForm) {
  signupForm.addEventListener("submit", async (e) => {
    const nonceField = signupForm.elements.pow_nonce;
    if (nonceField.value) {
      return; // solved: let the browser send the form
    }
    e.preventDefault();
    const button = signupForm.querySelector('button[type="submit"]');
    const label = button.textContent;
    button.disabled = true;
    button.textContent = "Checking that you are human…";
    const errorLine = document.getElementById("signup-check-error");
    errorLine.textContent = "";
    try {
      if (!window.crypto || !crypto.subtle) {
        throw new Error("no crypto.subtle");
      }
      nonceField.value = await solve(signupForm.elements.form_token.value, Number(signupForm.dataset.powBits));
      signupForm.submit();
    } catch (err) {
      button.disabled = false;
      button.textContent = label;
      errorLine.textContent = "Your browser could not run the sign-up check. Please use an up-to-date browser over HTTPS.";
    }
  });
}

async function solve(token, powBits) {
  const encoder = new TextEncoder();
  for (let start = 0; ; start += 256) {
    const tries = [];
    for (let i = start; i < start + 256; i++) {
      tries.push(crypto.subtle.digest("SHA-256", encoder.encode(token + ":" + i))
        .then((hash) => (zeroBits(new Uint8Array(hash)) >= powBits ? i : -1)));
    }
    const found = (await Promise.all(tries)).find((n) => n >= 0);
    if (found !== undefined) {
      return String(found);
    }
  }
}

function zeroBits(bytes) {
  let n = 0;
  for (const b of bytes) {
    if (b === 0) {
      n += 8;
      continue;
    }
    return n + Math.clz32(b) - 24;
  }
  return n;
}
