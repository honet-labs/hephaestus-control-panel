# Responsive UI and Docker sharing regression checks

## Delivery status

Implemented in `1271bc8` and present in `dev`: shared Docker connections do not
expose private containers through lists or summary counts; navigation includes
App Services → OpenSearch; mobile/tablet layout uses the responsive drawer and
page/table refinements described below. Docker policy/count regression tests
were rerun successfully on 20 November 2026. Final code commit `9f34c3a` also
passed [main CI 44698593173](https://github.com/honet-labs/hephaestus-control-panel/actions/runs/44698593173).
This is source/build/test delivery, not confirmation of production deployment or
completed visual testing on physical mobile/tablet devices. Production deployment
requires authorized access and remains outside the executed checks.

## Navigation and responsive layout

- Below 1280px the sidebar is an off-canvas drawer, leaving the full viewport
  available to the active page. At 1280px and above it remains collapsible.
- Check widths 320, 375, 768, 1024, and 1440px in light and dark themes.
- Open/close navigation using its buttons, backdrop, and Escape. Keyboard focus
  must remain in the open drawer; hidden navigation must not receive focus.
- Rotate/resize with navigation open. The backdrop must not remain over desktop
  content and the drawer must not reserve horizontal space on tablets.
- On Monitoring Instances, check both server/container tabs, grid/list views,
  filters, pagination, settings, and sharing dialogs. Wide tables should scroll
  inside their own region rather than widening the whole page.
- On Management Containers, check long connection names, summary cards, section
  tabs, visibility/state filters, and the container table on narrow screens.
- OpenSearch is under **App Services**, not Infrastructure. Existing URLs and
  the `opensearch` RBAC permission remain unchanged. Users without that permission
  should not see the group. Quick Search uses the same grouping label.

## Docker isolation

1. Create users A and B and an engine connection owned by the administrator.
2. Share only the connection with A. With no public or individually shared
   containers, A's list, total, running, paused, and stopped counts must be zero.
3. Deploy containers for A and B, including running, paused, and stopped states.
   A must not see B's private containers or their counts.
4. Share one of B's containers with A as `read`, then as `manage`. It should
   appear once in the list and summary. Read sharing must not allow mutations.
5. Revoke that share. The container must disappear from both list and counters.
6. Explicitly public containers retain the existing public/read-only policy;
   sharing a connection does not make its private containers public.
7. Administrators continue to see all containers. Search and UI filters must not
   change the authorized summary totals.
8. Verify `/api/v1/docker/system/info` and `/api/v1/docker/containers` directly:
   inaccessible connection IDs must return 403. Omitting the connection ID must
   select an accessible active connection, never another user's default engine.
9. Change connections during a refresh. Late responses from the old engine must
   not replace the new list or summary. Failed requests must not retain old totals.

The shared list/summary visibility policy and count reset behavior are covered by
`internal/handlers/docker_access_test.go` (`go test ./internal/handlers`).
