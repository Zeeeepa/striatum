ALTER TABLE striatumd.rpc_methods
  DROP CONSTRAINT IF EXISTS rpc_methods_required_capability_check;

ALTER TABLE striatumd.rpc_methods
  ADD CONSTRAINT rpc_methods_required_capability_check
  CHECK (required_capability IN (
    'read',
    'write',
    'review',
    'claim',
    'apply',
    'admin',
    'recovery',
    'surgical_recovery'
  ));

ALTER TABLE striatumd.client_capabilities
  DROP CONSTRAINT IF EXISTS client_capabilities_capability_check;

ALTER TABLE striatumd.client_capabilities
  ADD CONSTRAINT client_capabilities_capability_check
  CHECK (capability IN (
    'read',
    'write',
    'review',
    'claim',
    'apply',
    'admin',
    'recovery',
    'surgical_recovery'
  ));
