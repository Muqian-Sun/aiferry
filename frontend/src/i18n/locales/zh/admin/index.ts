import overview from './overview'
import channels from './channels'
import accounts from './accounts'
import resources from './resources'
import ops from './ops'
import settings from './settings'
import promptAudit from './promptAudit'
import modelCatalog from './modelCatalog'
import entity from './entity'

export default {
  ...overview,
  ...channels,
  ...accounts,
  ...resources,
  ...ops,
  ...settings,
  ...promptAudit,
  ...modelCatalog,
  ...entity,
}
