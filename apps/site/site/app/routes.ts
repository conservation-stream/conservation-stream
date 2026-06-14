import { type RouteConfig, route } from '@react-router/dev/routes';

export default [
  route('/', 'pages/Home.tsx'),
  route('/docs/:collection', 'pages/Docs.tsx', [route(':slug', 'pages/Main.tsx')])
] satisfies RouteConfig;
