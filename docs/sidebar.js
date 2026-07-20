(function () {
  var packages = [
    { name: "docker-image-merge", path: "/" },
    { name: "cmd", path: "/cmd/" },
    { name: "internal/docker", path: "/internal/docker/" },
    { name: "internal/flags", path: "/internal/flags/" },
    { name: "internal/merge", path: "/internal/merge/" },
    { name: "internal/tui", path: "/internal/tui/" }
  ];

  var links = [
    { name: "Installation", path: "/installation.md" }
  ];

  var current = window.location.pathname;
  // Normalize: remove trailing index.html
  current = current.replace(/index\.html$/, "");

  var sidebar = document.createElement("nav");
  sidebar.className = "sidebar";

  var title = document.createElement("div");
  title.className = "sidebar-title";
  title.textContent = "docker-image-merge";
  sidebar.appendChild(title);

  // Packages section
  var pkgSection = document.createElement("div");
  pkgSection.className = "sidebar-section";
  var pkgTitle = document.createElement("div");
  pkgTitle.className = "sidebar-section-title";
  pkgTitle.textContent = "Packages";
  pkgSection.appendChild(pkgTitle);

  packages.forEach(function (pkg) {
    var a = document.createElement("a");
    a.href = pkg.path;
    a.textContent = pkg.name;
    if (current === pkg.path || current === pkg.path.replace(/\/$/, "")) {
      a.className = "active";
    }
    pkgSection.appendChild(a);
  });
  sidebar.appendChild(pkgSection);

  // Links section
  var linkSection = document.createElement("div");
  linkSection.className = "sidebar-section";
  var linkTitle = document.createElement("div");
  linkTitle.className = "sidebar-section-title";
  linkTitle.textContent = "Guide";
  linkSection.appendChild(linkTitle);

  links.forEach(function (link) {
    var a = document.createElement("a");
    a.href = link.path;
    a.textContent = link.name;
    if (current === link.path) {
      a.className = "active";
    }
    linkSection.appendChild(a);
  });
  sidebar.appendChild(linkSection);

  document.body.insertBefore(sidebar, document.body.firstChild);
})();
