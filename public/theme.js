(() => {
	const root = document.documentElement;
	const settings = document.querySelector(".settings-menu");
	const choices = document.querySelectorAll('input[name="theme"]');
	const key = "vpsf-status.theme";
	function applyTheme(value) {
		const theme = value === "light" || value === "dark" ? value : "system";
		if (theme === "system") delete root.dataset.theme;
		else root.dataset.theme = theme;
		choices.forEach((choice) => { choice.checked = choice.value === theme; });
	}
	applyTheme(root.dataset.theme);
	choices.forEach((choice) => {
		choice.addEventListener("change", () => {
			if (!choice.checked) return;
			applyTheme(choice.value);
			try {
				if (choice.value === "system") localStorage.removeItem(key);
				else localStorage.setItem(key, choice.value);
			} catch (_) { /* Switching still works for this page. */ }
		});
	});
	if (settings) {
		settings.addEventListener("focusout", () => {
			requestAnimationFrame(() => {
				if (!settings.contains(document.activeElement)) settings.open = false;
			});
		});
		settings.querySelector(".settings-enhanced").hidden = false;
	}
	window.addEventListener("storage", (event) => {
		if (event.key === key || event.key === null) applyTheme(event.newValue);
	});

	const coarse = window.matchMedia("(any-pointer: coarse)");
	function scrollHistoryToRecent() {
		if (!coarse.matches) return;
		document.querySelectorAll(".history-track").forEach((track) => {
			if (!track.dataset.positioned && track.clientWidth) {
				track.scrollLeft = track.scrollWidth;
				track.dataset.positioned = "true";
			}
		});
	}
	scrollHistoryToRecent();
	document.addEventListener("shown.bs.collapse", scrollHistoryToRecent);
	coarse.addEventListener("change", scrollHistoryToRecent);

	// A single open popover supports hover, focus and touch; Escape/outside tap dismiss it.
	root.classList.add("history-enhanced");
	let active = null;
	let closeTimer = 0;
	let frame = 0;
	function close() {
		clearTimeout(closeTimer);
		if (active) active.classList.remove("is-open");
		active = null;
	}
	function position() {
		if (!active) return;
		const popup = active.querySelector(".history-popover");
		const rect = active.getBoundingClientRect();
		const bounds = popup.getBoundingClientRect();
		const clamp = (value, size, max) => Math.max(8, Math.min(value, max - size - 8));
		const top = rect.top - bounds.height - 8;
		popup.style.left = `${clamp(rect.left + rect.width / 2 - bounds.width / 2, bounds.width, window.innerWidth)}px`;
		popup.style.top = `${clamp(top >= 8 ? top : rect.bottom + 8, bounds.height, window.innerHeight)}px`;
	}
	function schedulePosition() {
		cancelAnimationFrame(frame);
		frame = requestAnimationFrame(position);
	}
	function open(wrap) {
		clearTimeout(closeTimer);
		if (active !== wrap) close();
		active = wrap;
		active.classList.add("is-open");
		position();
	}
	document.querySelectorAll(".history-day-wrap").forEach((wrap) => {
		wrap.addEventListener("pointerenter", (event) => {
			if (event.pointerType !== "touch") open(wrap);
		});
		wrap.addEventListener("pointerleave", () => {
			closeTimer = setTimeout(() => {
				if (active === wrap && !wrap.matches(":focus-within")) close();
			}, 180);
		});
		wrap.addEventListener("focusin", () => open(wrap));
		wrap.addEventListener("focusout", () => {
			requestAnimationFrame(() => {
				if (active === wrap && !wrap.matches(":focus-within") && !wrap.matches(":hover")) close();
			});
		});
		wrap.addEventListener("click", (event) => {
			if (!event.target.closest("a")) open(wrap);
		});
	});
	document.addEventListener("pointerdown", (event) => {
		if (active && !active.contains(event.target)) close();
		if (settings && !settings.contains(event.target)) settings.open = false;
	});
	document.addEventListener("keydown", (event) => {
		if (event.key === "Escape") {
			if (active && active.contains(document.activeElement)) active.focus();
			close();
			if (settings && settings.open) {
				settings.open = false;
				settings.querySelector("summary").focus();
			}
		}
	});
	window.addEventListener("resize", schedulePosition);
	window.addEventListener("scroll", schedulePosition, true);
})();
