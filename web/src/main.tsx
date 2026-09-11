import React from 'react'
import ReactDOM from 'react-dom/client'
import { createBrowserRouter, RouterProvider } from 'react-router-dom'
import App from './App'
import Overview from './pages/Overview'
import Agents from './pages/Agents'
import Links from './pages/Links'
import LinkDetail from './pages/LinkDetail'
import Settings from './pages/Settings'
import './index.css'

const router = createBrowserRouter([
  {
    path: '/',
    element: <App />,
    children: [
      { index: true, element: <Overview /> },
      { path: 'agents', element: <Agents /> },
      { path: 'links', element: <Links /> },
      { path: 'links/:linkId', element: <LinkDetail /> },
      { path: 'settings', element: <Settings /> },
    ],
  },
])

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <RouterProvider router={router} />
  </React.StrictMode>,
)
