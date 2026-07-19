(function () {
  const pages = Array.from(document.querySelectorAll('.doc-page'));
  const navLinks = Array.from(document.querySelectorAll('[data-route]'));
  const sidebar = document.getElementById('sidebar');
  const backdrop = document.getElementById('sidebar-backdrop');
  const menuToggle = document.getElementById('menu-toggle');
  const themeToggle = document.getElementById('theme-toggle');
  const outline = document.getElementById('page-outline-nav');
  const searchDialog = document.getElementById('search-dialog');
  const searchTrigger = document.getElementById('search-trigger');
  const searchClose = document.getElementById('search-close');
  const searchInput = document.getElementById('search-input');
  const searchResults = document.getElementById('search-results');
  const toast = document.getElementById('toast');
  const defaultRoute = 'overview';

  const normalize = (value) => value.toLowerCase().replace(/\s+/g, ' ').trim();
  const pageIndex = pages.map((page) => ({
    route: page.dataset.page,
    title: page.dataset.title,
    text: normalize(page.textContent || ''),
    description: page.querySelector('.lead')?.textContent || ''
  }));

  function currentRoute() {
    const requested = window.location.hash.replace(/^#/, '').split('/')[0];
    const value = requested === 'codex' ? 'ccswitch' : requested;
    return pages.some((page) => page.dataset.page === value) ? value : defaultRoute;
  }

  function closeSidebar() {
    sidebar.classList.remove('open');
    backdrop.classList.remove('open');
    menuToggle.setAttribute('aria-expanded', 'false');
  }

  function buildOutline(page) {
    outline.replaceChildren();
    page.querySelectorAll('h2').forEach((heading, index) => {
      const id = `${page.dataset.page}-section-${index + 1}`;
      heading.id = id;
      const link = document.createElement('a');
      link.href = `#${id}`;
      link.textContent = heading.textContent;
      link.addEventListener('click', (event) => {
        event.preventDefault();
        heading.scrollIntoView({ behavior: 'smooth', block: 'start' });
      });
      outline.appendChild(link);
    });
  }

  function renderRoute() {
    const route = currentRoute();
    let activePage;
    pages.forEach((page) => {
      const active = page.dataset.page === route;
      page.classList.toggle('active', active);
      page.setAttribute('aria-hidden', String(!active));
      if (active) activePage = page;
    });
    navLinks.forEach((link) => {
      const active = link.dataset.route === route;
      link.classList.toggle('active', active);
      if (active) link.setAttribute('aria-current', 'page');
      else link.removeAttribute('aria-current');
    });
    if (activePage) {
      document.title = `${activePage.dataset.title} - ChatAPI 使用手册`;
      buildOutline(activePage);
    }
    closeSidebar();
    window.scrollTo({ top: 0, behavior: 'auto' });
  }

  function setTheme(theme) {
    document.documentElement.dataset.theme = theme;
    localStorage.setItem('chatapi-docs-theme', theme);
    themeToggle.textContent = theme === 'dark' ? '☀' : '◐';
  }

  function showToast(message) {
    toast.textContent = message;
    toast.classList.add('show');
    window.clearTimeout(showToast.timer);
    showToast.timer = window.setTimeout(() => toast.classList.remove('show'), 1500);
  }

  function renderSearch(query) {
    const needle = normalize(query);
    const matches = needle
      ? pageIndex.filter((item) => item.title.toLowerCase().includes(needle) || item.text.includes(needle)).slice(0, 10)
      : pageIndex.slice(0, 8);
    searchResults.replaceChildren();
    if (!matches.length) {
      const empty = document.createElement('p');
      empty.className = 'search-empty';
      empty.textContent = '没有找到匹配内容';
      searchResults.appendChild(empty);
      return;
    }
    matches.forEach((item) => {
      const link = document.createElement('a');
      link.className = 'search-result';
      link.href = `#${item.route}`;
      const title = document.createElement('strong');
      title.textContent = item.title;
      const description = document.createElement('small');
      description.textContent = item.description;
      link.append(title, description);
      link.addEventListener('click', () => searchDialog.close());
      searchResults.appendChild(link);
    });
  }

  menuToggle.setAttribute('aria-expanded', 'false');
  menuToggle.addEventListener('click', () => {
    const open = !sidebar.classList.contains('open');
    sidebar.classList.toggle('open', open);
    backdrop.classList.toggle('open', open);
    menuToggle.setAttribute('aria-expanded', String(open));
  });
  backdrop.addEventListener('click', closeSidebar);

  const preferredTheme = localStorage.getItem('chatapi-docs-theme') ||
    (window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light');
  setTheme(preferredTheme);
  themeToggle.addEventListener('click', () => {
    setTheme(document.documentElement.dataset.theme === 'dark' ? 'light' : 'dark');
  });

  searchTrigger.addEventListener('click', () => {
    renderSearch('');
    searchDialog.showModal();
    searchInput.focus();
  });
  searchClose.addEventListener('click', () => searchDialog.close());
  searchInput.addEventListener('input', () => renderSearch(searchInput.value));
  searchDialog.addEventListener('click', (event) => {
    if (event.target === searchDialog) searchDialog.close();
  });

  document.querySelectorAll('.copy-button').forEach((button) => {
    button.addEventListener('click', async () => {
      const value = button.closest('.code-block').querySelector('code').textContent;
      try {
        await navigator.clipboard.writeText(value);
        showToast('代码已复制');
      } catch {
        const textarea = document.createElement('textarea');
        textarea.value = value;
        textarea.style.position = 'fixed';
        textarea.style.opacity = '0';
        document.body.appendChild(textarea);
        textarea.select();
        const copied = document.execCommand('copy');
        textarea.remove();
        showToast(copied ? '代码已复制' : '复制失败，请手动选择');
      }
    });
  });

  window.addEventListener('hashchange', renderRoute);
  renderRoute();
})();
