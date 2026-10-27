import React, { useEffect, useState } from 'react';
import { createRoot } from 'react-dom/client';
import './styles.css';

const icons = { banana: '🍌', apple: '🍎', orange: '🍊', melon: '🍉' };
const money = value => new Intl.NumberFormat('en-US', { style: 'currency', currency: 'USD' }).format(value);
const blankFruit = { name: '', description: '', icon: 'banana', price: '', stock: '' };

async function api(path, { password, ...options } = {}) {
  const response = await fetch(`/api${path}`, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      ...(password ? { 'X-Admin-Password': password } : {}),
    },
  });
  if (!response.ok) {
    const problem = await response.json().catch(() => null);
    throw new Error(problem?.detail || `The shop could not complete that request (${response.status}). Please try again.`);
  }
  return response.status === 204 ? null : response.json();
}

function App() {
  const [products, setProducts] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const [busy, setBusy] = useState(false);
  const [tab, setTab] = useState('shop');
  const [password, setPassword] = useState('');
  const [unlocked, setUnlocked] = useState(false);
  const [fruit, setFruit] = useState(blankFruit);
  const [receipt, setReceipt] = useState(null);

  async function load() {
    setLoading(true);
    setError('');
    try { setProducts(await api('/products')); }
    catch (e) { setError(e.message); }
    finally { setLoading(false); }
  }
  useEffect(() => { load(); }, []);

  async function act(work) {
    setBusy(true);
    setError('');
    setNotice('');
    try { await work(); }
    catch (e) { setError(e.message); }
    finally { setBusy(false); }
  }
  async function refresh() {
    // Let mutation handlers report a refresh failure without implying the write failed.
    try { setProducts(await api('/products')); }
    catch (e) { setError(`Your change was saved, but inventory could not refresh. ${e.message}`); }
  }
  function buy(product) {
    act(async () => {
      setReceipt(await api('/orders', { method: 'POST', body: JSON.stringify({ productId: product.id }) }));
      await refresh();
    });
  }
  function add(event) {
    event.preventDefault();
    act(async () => {
      await api('/admin/products', {
        method: 'POST', password,
        body: JSON.stringify({ ...fruit, price: Number(fruit.price), stock: Number(fruit.stock) }),
      });
      setFruit(blankFruit);
      setNotice('Fruit added. Your shelves are looking good!');
      await refresh();
    });
  }

  return (
    <div className="shell">
      <div className="topline"><span>THE WORLD WIDE WEB'S TALLEST FRUIT STAND</span><span>EST. 1996 ✦ ALWAYS FRESH</span></div>
      <header>
        <a className="brand" href="#" aria-label="Tall Order home" onClick={e => { e.preventDefault(); setTab('shop'); }}>
          <span className="brand-icon" aria-hidden="true">🦒</span>
          <span>TALL ORDER<small>GOOD FRUIT. GREAT HEIGHTS.</small></span>
        </a>
        <nav aria-label="Main navigation">
          <button className={tab === 'shop' ? 'selected' : ''} onClick={() => setTab('shop')}>Fruit stand</button>
          <button className={tab === 'admin' ? 'selected' : ''} onClick={() => setTab('admin')}>Staff room ↗</button>
        </nav>
      </header>

      <main>
        <section className="hero">
          <div className="hero-copy">
            <span className="eyebrow">★ FRESH PICKS FOR LONG NECKS ★</span>
            <h1>A little fruit.<br />A <em>tall</em> order.</h1>
            <p>Welcome to your neck of the woods. Tree-ripened treats for giraffes with exceptionally good taste.</p>
            <a className="button primary" href="#inventory" onClick={() => setTab('shop')}>Browse the good stuff ↓</a>
            <div className="hero-note">No ladders required. Just a healthy appetite.</div>
          </div>
          <div className="hero-art" aria-label="Giraffe with a basket of fruit" role="img">
            <div className="sunburst">100%<br /><strong>NECK-TAR</strong><br />OF THE GODS</div>
            <span className="giraffe">🦒</span>
            <span className="fruit-pile">🍌 🍎 🍊</span>
            <span className="art-caption">a balanced diet is fruit in every spot.</span>
          </div>
        </section>
        <div className="ticker" aria-hidden="true">FRESH OFF THE BRANCH ✦ SMALL PRICES, TALL STANDARDS ✦ GIRAFFE APPROVED ✦ FRESH OFF THE BRANCH</div>

        <section id="inventory" className="inventory">
          <div className="section-heading">
            <div><span className="eyebrow">{tab === 'shop' ? 'THE DAILY GRAZE' : 'EMPLOYEES WITH LONG NECKS ONLY'}</span><h2>{tab === 'shop' ? 'Pick your happy.' : 'The staff room.'}</h2></div>
            <span className="stamp">{tab === 'shop' ? 'BRANCH TO BASKET' : 'INVENTORY CONTROL'}</span>
          </div>
          {error && <div role="alert" className="message error">{error} <button onClick={load} disabled={busy}>Reload inventory</button></div>}
          {notice && <div role="status" className="message">{notice}</div>}
          {receipt && <div role="status" className="receipt">
            <span>✓ ORDER PLANTED!</span><strong>{receipt.productName} · {money(receipt.total)}</strong>
            <small>Receipt #{receipt.id.slice(0, 8)} · One fruit reserved. No real payment taken.</small>
            <button onClick={() => setReceipt(null)} aria-label="Dismiss receipt">×</button>
          </div>}
          {tab === 'admin' && !unlocked ? (
            <form className="admin-form unlock" onSubmit={event => {
              event.preventDefault();
              act(async () => {
                await api('/admin/unlock', { method: 'POST', password });
                setUnlocked(true);
              });
            }}>
              <h3>Welcome back, branch manager.</h3>
              <p>This is a local demo lock, not production authentication.</p>
              <label>Demo password<input type="password" value={password} onChange={e => setPassword(e.target.value)} required autoComplete="off" /></label>
              <button className="button primary" disabled={busy}>{busy ? 'Checking…' : 'Unlock staff room'}</button>
            </form>
          ) : (
            <>
              {tab === 'admin' && <form className="admin-form" onSubmit={add}>
                <div className="form-heading"><h3>Put something fresh on the shelf.</h3><button type="button" onClick={() => { setUnlocked(false); setPassword(''); }}>Lock staff room</button></div>
                <div className="fields">
                  <label>Fruit name<input required maxLength={60} value={fruit.name} onChange={e => setFruit({ ...fruit, name: e.target.value })} /></label>
                  <label>Price (USD)<input type="number" required min="0.01" max="999999.99" step="0.01" value={fruit.price} onChange={e => setFruit({ ...fruit, price: e.target.value })} /></label>
                  <label>Stock<input type="number" required min="0" max="10000" step="1" value={fruit.stock} onChange={e => setFruit({ ...fruit, stock: e.target.value })} /></label>
                  <label>Fruit icon<select value={fruit.icon} onChange={e => setFruit({ ...fruit, icon: e.target.value })}>{Object.entries(icons).map(([key, icon]) => <option key={key} value={key}>{icon} {key}</option>)}</select></label>
                  <label className="description">Description<input required maxLength={180} value={fruit.description} onChange={e => setFruit({ ...fruit, description: e.target.value })} /></label>
                </div>
                <button className="button primary" disabled={busy}>{busy ? 'Saving…' : '+ Add fruit'}</button>
              </form>}
              {loading ? <div role="status" className="empty">Picking the freshest fruit…</div>
                : products.length === 0 ? <div className="empty"><h3>The branches are bare.</h3><p>{error ? 'Inventory is unavailable. Try reloading above.' : 'The branch manager will restock soon. Check back for fresh picks!'}</p></div>
                  : <div className="product-grid">{products.map(product => (
                    <article className={`product ${product.icon}`} key={product.id}>
                      <div className="product-art"><span aria-hidden="true">{icons[product.icon]}</span><span className="stock">{product.stock > 0 ? `${product.stock} on the branch` : 'SOLD OUT'}</span></div>
                      <div className="product-body"><h3>{product.name}</h3><p>{product.description}</p>
                        <div className="product-bottom"><strong>{money(product.price)}<small> / each</small></strong>
                          {tab === 'admin'
                            ? <button className="button remove" disabled={busy} onClick={() => {
                              if (window.confirm(`Remove ${product.name} from the store?`)) act(async () => {
                                await api(`/admin/products/${product.id}`, { method: 'DELETE', password });
                                setNotice(`${product.name} removed from the store.`);
                                await refresh();
                              });
                            }}>Remove</button>
                            : <button className="button buy" disabled={busy || product.stock === 0} onClick={() => buy(product)}> {product.stock === 0 ? 'Sold out' : 'Buy one ↗'}</button>}
                        </div>
                      </div>
                    </article>
                  ))}</div>}
            </>
          )}
        </section>
        <aside className="promise"><span aria-hidden="true">✳</span><div><h2>Life's better at the top.</h2><p>Good fruit, honest prices, and absolutely no short-changing.</p></div><span className="badge">GIRAFFE<br />APPROVED<br />✓</span></aside>
      </main>
      <footer><strong>TALL ORDER © 1996–2026</strong><span>Made for the high life. Best viewed with a long neck.</span><span className="counter">VISITORS 000042</span></footer>
    </div>
  );
}

createRoot(document.getElementById('root')).render(<App />);
