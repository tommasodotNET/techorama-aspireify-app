import './style.css';

const $ = (selector) => document.querySelector(selector);
const money = new Intl.NumberFormat('en-US', { style: 'currency', currency: 'USD' });
let books = [];
let credentials = '';
let loaded = false;
let busy = false;

async function api(path, options = {}) {
  const response = await fetch(path, {
    ...options,
    headers: {
      ...(options.body ? { 'Content-Type': 'application/json' } : {}),
      ...(credentials ? { Authorization: `Basic ${credentials}` } : {}),
    },
    signal: AbortSignal.timeout(15000),
  });
  if (!response.ok) {
    const body = await response.json();
    if (response.status === 401) lock();
    throw new Error(body.error || `Request failed (${response.status}).`);
  }
  return response.status === 204 ? null : response.json();
}

function errorMessage(error) {
  if (error.name === 'TimeoutError') return 'The request timed out. Check the shelf before retrying.';
  if (error instanceof TypeError || error instanceof SyntaxError) return 'The bookshop is offline. Please try again.';
  return error.message;
}

function node(tag, className, text) {
  const element = document.createElement(tag);
  if (className) element.className = className;
  if (text !== undefined) element.textContent = text;
  return element;
}

function render() {
  const query = $('#search').value.trim().toLowerCase();
  const visible = books.filter((book) => `${book.title} ${book.author} ${book.seller}`.toLowerCase().includes(query));
  $('#book-count').textContent = `${books.length} second-paw treasures`;
  $('#catalog').replaceChildren();
  $('#admin-list').replaceChildren();
  if (loaded) $('#catalog-status').textContent = visible.length ? '' : query ? 'No books match your search. Try another title or cat.' : 'The shelf is empty. Our cats are out finding their next great read.';
  visible.forEach((book, index) => {
    const card = node('article', 'book-card');
    const cover = node('div', `cover cover-${index % 4}`);
    cover.append(node('span', 'cover-label', 'THE FELINE EDITION'), node('h3', '', book.title), node('span', 'cover-cat', '=^.^='), node('span', 'cover-author', book.author));
    const details = node('div', 'book-details');
    details.append(node('p', 'seller', `FROM THE SHELF OF ${book.seller.toUpperCase()}`), node('h3', '', book.title), node('p', 'author', `by ${book.author}`), node('p', 'description', book.description || 'A second-paw treasure, carefully selected by a very good cat.'), node('p', 'price', money.format(book.priceCents / 100)));
    card.append(cover, details);
    $('#catalog').append(card);
  });
  if (loaded && !books.length) $('#admin-list').append(node('p', '', 'No listings yet. Add your first book above.'));
  books.forEach((book) => {
    const row = node('div', 'admin-row');
    const text = node('div');
    text.append(node('strong', '', book.title), node('p', '', `${book.seller} / ${money.format(book.priceCents / 100)}`));
    const remove = node('button', 'secondary', 'Remove');
    remove.setAttribute('aria-label', `Remove ${book.title}`);
    remove.disabled = busy;
    remove.addEventListener('click', () => removeBook(book));
    row.append(text, remove);
    $('#admin-list').append(row);
  });
}

async function loadBooks() {
  $('#catalog-error').hidden = true;
  $('#catalog-status').textContent = 'The cats are fetching the books...';
  $('#admin-error').textContent = '';
  $('#retry').disabled = true;
  $('#admin-refresh').disabled = true;
  try {
    books = await api('/api/books');
    loaded = true;
    render();
  } catch (error) {
    loaded = false;
    books = [];
    render();
    $('#book-count').textContent = 'Shelf offline';
    $('#catalog-status').textContent = '';
    $('#catalog-error').hidden = false;
    $('#catalog-error p').textContent = errorMessage(error);
    $('#admin-error').textContent = errorMessage(error);
  } finally {
    $('#retry').disabled = false;
    $('#admin-refresh').disabled = false;
  }
}

function route() {
  const admin = location.hash === '#admin';
  $('#shop-view').hidden = admin;
  $('#admin-view').hidden = !admin;
  $('#shop-link').setAttribute('aria-current', admin ? 'false' : 'page');
  $('#admin-link').setAttribute('aria-current', admin ? 'page' : 'false');
}

function lock() {
  credentials = '';
  $('#login-form').hidden = false;
  $('#admin-tools').hidden = true;
  $('#password').value = '';
  $('#book-form').reset();
  $('#admin-message').textContent = '';
  $('#admin-error').textContent = '';
}

function setBusy(value) {
  busy = value;
  $('#book-form button').disabled = value;
  $('#logout').disabled = value;
  $('#admin-refresh').disabled = value;
  render();
}

$('#login-form').addEventListener('submit', async (event) => {
  event.preventDefault();
  const button = $('#login-form button');
  button.disabled = true;
  $('#login-error').textContent = '';
  // Credentials stay in memory only; refreshing or locking the portal discards them.
  credentials = btoa(String.fromCharCode(...new TextEncoder().encode(`admin:${$('#password').value}`)));
  try {
    await api('/api/admin/check', { method: 'POST' });
    $('#password').value = '';
    $('#login-form').hidden = true;
    $('#admin-tools').hidden = false;
    await loadBooks();
  } catch (error) {
    lock();
    $('#login-error').textContent = errorMessage(error);
  } finally {
    button.disabled = false;
  }
});

$('#book-form').addEventListener('submit', async (event) => {
  event.preventDefault();
  if (busy) return;
  const form = new FormData(event.currentTarget);
  const book = {
    title: form.get('title'), author: form.get('author'), seller: form.get('seller'),
    priceCents: Math.round(Number(form.get('price')) * 100), description: form.get('description'),
  };
  $('#admin-error').textContent = '';
  $('#admin-message').textContent = 'Saving to the bookshelf...';
  setBusy(true);
  try {
    const saved = await api('/api/books', { method: 'POST', body: JSON.stringify(book) });
    books.push(saved);
    loaded = true;
    $('#book-form').reset();
    $('#admin-message').textContent = `"${saved.title}" is now on the public shelf.`;
  } catch (error) {
    $('#admin-message').textContent = '';
    $('#admin-error').textContent = errorMessage(error);
  } finally {
    setBusy(false);
  }
});

async function removeBook(book) {
  if (busy || !window.confirm(`Remove "${book.title}" from the bookshelf?`)) return;
  $('#admin-error').textContent = '';
  $('#admin-message').textContent = 'Removing the listing...';
  setBusy(true);
  try {
    await api(`/api/books/${encodeURIComponent(book.id)}`, { method: 'DELETE' });
    books = books.filter((item) => item.id !== book.id);
    $('#admin-message').textContent = `"${book.title}" was removed.`;
  } catch (error) {
    $('#admin-message').textContent = '';
    $('#admin-error').textContent = errorMessage(error);
  } finally {
    setBusy(false);
  }
}

$('#logout').addEventListener('click', lock);
$('#search').addEventListener('input', render);
$('#retry').addEventListener('click', loadBooks);
$('#admin-refresh').addEventListener('click', loadBooks);
window.addEventListener('hashchange', route);
route();
loadBooks();
