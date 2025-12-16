
import { useState, useEffect } from 'react';
import { Terminal, Database, Wrench, RefreshCw, AlertCircle, FileText, ChevronRight, ChevronDown, Monitor } from 'lucide-react';
import './App.css';

const API_BASE = "http://localhost:8080";
const AUTH_TOKEN = "TEST"; // Hardcoded for demo, normally env var or login

function App() {
  const [tools, setTools] = useState([]);
  const [resources, setResources] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [activeTab, setActiveTab] = useState('tools'); // 'tools' | 'resources'
  const [expandedItems, setExpandedItems] = useState({});

  const fetchData = async () => {
    setLoading(true);
    setError(null);
    try {
      // Parallel fetch
      const [toolsRes, resourcesRes] = await Promise.all([
        fetch(`${API_BASE}/tools/list`, {
          headers: { "Authorization": `Bearer ${AUTH_TOKEN}` }
        }),
        fetch(`${API_BASE}/resources/list`, {
          headers: { "Authorization": `Bearer ${AUTH_TOKEN}` }
        })
      ]);

      if (!toolsRes.ok) throw new Error(`Tools API Error: ${toolsRes.status}`);
      const toolsData = await toolsRes.json();
      setTools(toolsData || []);

      if (!resourcesRes.ok) throw new Error(`Resources API Error: ${resourcesRes.status}`);
      const resourcesData = await resourcesRes.json();
      setResources(resourcesData.resources || []); // API returns {resources: [...]}

    } catch (err) {
      console.error(err);
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchData();
  }, []);

  const toggleExpand = (id) => {
    setExpandedItems(prev => ({ ...prev, [id]: !prev[id] }));
  };

  // Group items by upstream
  const groupByUpstream = (items) => {
    const groups = {};
    items.forEach(item => {
      const up = item.upstream || 'unknown';
      if (!groups[up]) groups[up] = [];
      groups[up].push(item);
    });
    return groups;
  };

  const toolsByUpstream = groupByUpstream(tools);
  const resourcesByUpstream = groupByUpstream(resources);

  return (
    <div className="dashboard">
      <header className="header">
        <div className="logo">
          <Monitor className="icon" />
          <h1>GoMCP Pilot</h1>
        </div>
        <button className="refresh-btn" onClick={fetchData} disabled={loading}>
          <RefreshCw className={`icon ${loading ? 'spin' : ''}`} />
          Refresh
        </button>
      </header>

      <div className="main-content">
        <aside className="sidebar">
          <nav>
            <button
              className={`nav-item ${activeTab === 'tools' ? 'active' : ''}`}
              onClick={() => setActiveTab('tools')}
            >
              <Wrench className="icon" /> Tools
              <span className="badge">{tools.length}</span>
            </button>
            <button
              className={`nav-item ${activeTab === 'resources' ? 'active' : ''}`}
              onClick={() => setActiveTab('resources')}
            >
              <Database className="icon" /> Resources
              <span className="badge">{resources.length}</span>
            </button>
          </nav>
        </aside>

        <main className="content-area">
          {error && (
            <div className="error-banner">
              <AlertCircle className="icon" />
              {error}
            </div>
          )}

          {activeTab === 'tools' && (
            <div className="section">
              <h2>Available Tools</h2>
              {Object.keys(toolsByUpstream).length === 0 && !loading && (
                <p className="empty-state">No tools found.</p>
              )}
              {Object.entries(toolsByUpstream).map(([upstream, groupTools]) => (
                <div key={upstream} className="upstream-group">
                  <h3 className="upstream-title"><Terminal className="icon-sm" /> {upstream}</h3>
                  <div className="grid">
                    {groupTools.map((tool) => {
                      const id = `${tool.upstream}-${tool.name}`;
                      const isExpanded = expandedItems[id];
                      return (
                        <div key={id} className={`card ${isExpanded ? 'expanded' : ''}`} onClick={() => toggleExpand(id)}>
                          <div className="card-header">
                            <span className="card-title">{tool.name}</span>
                            {isExpanded ? <ChevronDown className="icon-sm" /> : <ChevronRight className="icon-sm" />}
                          </div>
                          <p className="card-desc">{tool.description || "No description provided."}</p>
                          {isExpanded && (
                            <div className="card-details">
                              <pre>{JSON.stringify(tool.input_schema, null, 2)}</pre>
                            </div>
                          )}
                        </div>
                      );
                    })}
                  </div>
                </div>
              ))}
            </div>
          )}

          {activeTab === 'resources' && (
            <div className="section">
              <h2>Available Resources</h2>
              {Object.keys(resourcesByUpstream).length === 0 && !loading && (
                <p className="empty-state">No resources found. (Check console logs if upstreams failed to list)</p>
              )}
              {Object.entries(resourcesByUpstream).map(([upstream, groupRes]) => (
                <div key={upstream} className="upstream-group">
                  <h3 className="upstream-title"><Database className="icon-sm" /> {upstream}</h3>
                  <div className="list">
                    {groupRes.map((res) => {
                      const id = `${res.upstream}-${res.name}`;
                      return (
                        <div key={id} className="list-item">
                          <div className="list-item-header">
                            <FileText className="icon" />
                            <div className="list-item-info">
                              <span className="res-name">{res.name}</span>
                              <span className="res-uri">{res.uri}</span>
                            </div>
                          </div>
                          {res.mimeType && <span className="badge-mime">{res.mimeType}</span>}
                        </div>
                      )
                    })}
                  </div>
                </div>
              ))}
            </div>
          )}
        </main>
      </div>
    </div>
  );
}

export default App;
