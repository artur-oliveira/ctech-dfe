import * as cdk from 'aws-cdk-lib';
import {RemovalPolicy} from 'aws-cdk-lib';
import * as dynamodb from 'aws-cdk-lib/aws-dynamodb';
import {Billing} from 'aws-cdk-lib/aws-dynamodb';
import {Construct} from 'constructs';
import {Environment} from "./types";

export type TableName = (
  'roles' |
  'users' |
  'organizations' |
  'organization_users' |
  'organization_invitations' |
  'account_billing' |
  'audit_logs' |
  'products' |
  'vehicles' |
  'persons' |
  'certificates' |
  'nfe_configs' |
  'nfce_configs' |
  'cte_configs' |
  'mdfe_configs' |
  'nfse_configs' |
  'services' |
  'tax_profiles' |
  'operations' |
  'payment_terms' |
  'vehicle_sets' |
  'nfes' |
  'nfces' |
  'ctes' |
  'mdfes' |
  'nfses' |
  'nfe_events' |
  'nfce_events' |
  'cte_events' |
  'mdfe_events' |
  'nfse_events' |
  'nfe_distributions' |
  'cte_distributions' |
  'mdfe_distributions' |
  'nfse_distributions' |
  'worker_outbox'
  )

type TablePrefix = `${Environment}_dfe`

interface DynamoDBStackProps extends cdk.StackProps {
  tablePrefix: TablePrefix;
  environment: Environment;
}

const getDfeTable = (
  scope: Construct,
  removalPolicy: RemovalPolicy,
  pointInTimeRecoverySpecification: dynamodb.PointInTimeRecoverySpecification | undefined,
  tbPrefix: TablePrefix,
  tbName: TableName
) => {
  const dfeTable = new dynamodb.TableV2(scope, `${tbPrefix}_${tbName}`, {
    tableName: `${tbPrefix}_${tbName}`,
    partitionKey: {name: 'pk', type: dynamodb.AttributeType.STRING},
    sortKey: {name: 'sk', type: dynamodb.AttributeType.STRING},
    billing: Billing.onDemand({
      maxReadRequestUnits: 1000,
      maxWriteRequestUnits: 1000,
    }),
    removalPolicy,
    pointInTimeRecoverySpecification,
    encryption: dynamodb.TableEncryptionV2.awsManagedKey(),
  });

  dfeTable.addGlobalSecondaryIndex({
    indexName: `number-index-v2`,
    partitionKey: {
      name: 'pk', type: dynamodb.AttributeType.STRING
    },
    sortKeys: [
      {
        name: 'number', type: dynamodb.AttributeType.NUMBER
      },
      {
        name: 'incoming', type: dynamodb.AttributeType.NUMBER
      }
    ],
    projectionType: dynamodb.ProjectionType.ALL,
    warmThroughput: undefined,
    maxReadRequestUnits: 1000,
    maxWriteRequestUnits: 1000,
  });

  dfeTable.addGlobalSecondaryIndex({
    indexName: 'dfe-index',
    partitionKeys: [
      {
        name: 'pk', type: dynamodb.AttributeType.STRING
      },
      {
        name: 'incoming', type: dynamodb.AttributeType.NUMBER
      },
    ],
    sortKeys: [
      {
        name: 'year', type: dynamodb.AttributeType.NUMBER
      },
      {
        name: 'month', type: dynamodb.AttributeType.NUMBER
      },
      {
        name: 'day', type: dynamodb.AttributeType.NUMBER
      },
      {
        name: 'number', type: dynamodb.AttributeType.NUMBER
      },
    ],
    projectionType: dynamodb.ProjectionType.ALL,
    warmThroughput: undefined,
    maxReadRequestUnits: 1000,
    maxWriteRequestUnits: 1000,
  });

  return dfeTable;
}

const getDistributionTable = (
  scope: Construct,
  removalPolicy: RemovalPolicy,
  pointInTimeRecoverySpecification: dynamodb.PointInTimeRecoverySpecification | undefined,
  tbPrefix: TablePrefix,
  tbName: TableName
) => {
  return new dynamodb.TableV2(
    scope,
    `${tbPrefix}_${tbName}`,
    {
      tableName: `${tbPrefix}_${tbName}`,
      partitionKey: {name: 'pk', type: dynamodb.AttributeType.STRING},
      sortKey: {name: 'nsu', type: dynamodb.AttributeType.NUMBER},
      billing: Billing.onDemand({
        maxReadRequestUnits: 1000,
        maxWriteRequestUnits: 1000,
      }),
      removalPolicy,
      pointInTimeRecoverySpecification,
      encryption: dynamodb.TableEncryptionV2.awsManagedKey(),
    }
  );
}

const getEventsTable = (
  scope: Construct,
  removalPolicy: RemovalPolicy,
  pointInTimeRecoverySpecification: dynamodb.PointInTimeRecoverySpecification | undefined,
  tbPrefix: TablePrefix,
  tbName: TableName
) => {
  const eventsTable = new dynamodb.TableV2(
    scope,
    `${tbPrefix}_${tbName}`,
    {
      tableName: `${tbPrefix}_${tbName}`,
      partitionKey: {
        name: 'pk',
        type: dynamodb.AttributeType.STRING,
      },
      sortKey: {
        name: 'sk',
        type: dynamodb.AttributeType.STRING,
      },
      billing: Billing.onDemand({
        maxReadRequestUnits: 1000,
        maxWriteRequestUnits: 1000,
      }),
      removalPolicy,
      pointInTimeRecoverySpecification: pointInTimeRecoverySpecification,
      encryption: dynamodb.TableEncryptionV2.awsManagedKey(),
    }
  );

  // org-access-key-index: pk=org_pk, sk=event_key ({access_key}#{event_type})
  // Allows querying all events of a document (begins_with access_key) or
  // events of a specific type (exact match on {access_key}#{event_type}).
  eventsTable.addGlobalSecondaryIndex({
    indexName: 'org-event-key-index',
    partitionKey: {
      name: 'pk', type: dynamodb.AttributeType.STRING
    },
    sortKey: {
      name: 'event_key', type: dynamodb.AttributeType.STRING
    },
    projectionType: dynamodb.ProjectionType.ALL,
    warmThroughput: undefined,
    maxReadRequestUnits: 1000,
    maxWriteRequestUnits: 1000,
  });
  return eventsTable;
}

/**
 * Cadastro reutilizável de organização: pk = {org_pk}, sk = {PREFIXO}_{uuid},
 * com um único GSI `name-index` para busca por prefixo de nome. É a forma dos
 * perfis fiscais, naturezas de operação, condições de pagamento e composições
 * veiculares — todas listadas e buscadas exatamente igual.
 *
 * Custo: on-demand, mesmo teto de 1.000 RRU/WRU das demais tabelas de cadastro.
 * Volume esperado por organização é de dezenas de itens, não milhares — a
 * entidade existe justamente para não ser recadastrada a cada emissão.
 */
const getOrgEntityTable = (
  scope: Construct,
  removalPolicy: RemovalPolicy,
  pointInTimeRecoverySpecification: dynamodb.PointInTimeRecoverySpecification | undefined,
  tbPrefix: TablePrefix,
  tbName: TableName
) => {
  const table = new dynamodb.TableV2(scope, `${tbPrefix}_organization_${tbName}`, {
    tableName: `${tbPrefix}_organization_${tbName}`,
    partitionKey: {name: 'pk', type: dynamodb.AttributeType.STRING},
    sortKey: {name: 'sk', type: dynamodb.AttributeType.STRING},
    billing: Billing.onDemand({
      maxReadRequestUnits: 1000,
      maxWriteRequestUnits: 1000,
    }),
    removalPolicy,
    pointInTimeRecoverySpecification,
    encryption: dynamodb.TableEncryptionV2.awsManagedKey(),
  });
  table.addGlobalSecondaryIndex({
    indexName: 'name-index',
    partitionKey: {name: 'pk', type: dynamodb.AttributeType.STRING},
    sortKey: {name: 'name', type: dynamodb.AttributeType.STRING},
    projectionType: dynamodb.ProjectionType.ALL,
    warmThroughput: undefined,
    maxReadRequestUnits: 1000,
    maxWriteRequestUnits: 1000,
  });
  return table;
}

const getDfeConfigTable = (
  scope: Construct,
  removalPolicy: RemovalPolicy,
  pointInTimeRecoverySpecification: dynamodb.PointInTimeRecoverySpecification | undefined,
  tbPrefix: TablePrefix,
  tbName: TableName
) => {
  return new dynamodb.TableV2(
    scope,
    `${tbPrefix}_${tbName}`,
    {
      tableName: `${tbPrefix}_organization_${tbName}`,
      partitionKey: {name: 'pk', type: dynamodb.AttributeType.STRING},
      billing: Billing.onDemand({
        maxReadRequestUnits: 1000,
        maxWriteRequestUnits: 1000,
      }),
      removalPolicy,
      pointInTimeRecoverySpecification,
      encryption: dynamodb.TableEncryptionV2.awsManagedKey(),
    }
  );
}

export class DynamoDBStack extends cdk.Stack {
  public readonly tables: Map<TableName, dynamodb.TableV2>;

  constructor(scope: Construct, id: string, props: DynamoDBStackProps) {
    super(scope, id, props);

    this.tables = new Map();
    const {tablePrefix, environment} = props;
    const removalPolicy = environment === 'dev' ? cdk.RemovalPolicy.DESTROY : cdk.RemovalPolicy.RETAIN;
    const pointInTimeRecoverySpecification = environment === 'prod' ? {pointInTimeRecoveryEnabled: true} : undefined;
    // ============== BASE TABLES ==============

    const rolesTable = new dynamodb.TableV2(this, `${tablePrefix}_roles`, {
      tableName: `${tablePrefix}_roles`,
      partitionKey: {name: 'pk', type: dynamodb.AttributeType.STRING},
      billing: Billing.onDemand({
        maxReadRequestUnits: 1000,
        maxWriteRequestUnits: 1000,
      }),
      removalPolicy,
      pointInTimeRecoverySpecification,
      encryption: dynamodb.TableEncryptionV2.awsManagedKey(),
    });
    this.tables.set('roles', rolesTable);

    const usersTable = new dynamodb.TableV2(this, `${tablePrefix}_users`, {
      tableName: `${tablePrefix}_users`,
      partitionKey: {name: 'pk', type: dynamodb.AttributeType.STRING},
      billing: Billing.onDemand({
        maxReadRequestUnits: 1000,
        maxWriteRequestUnits: 1000,
      }),
      removalPolicy,
      pointInTimeRecoverySpecification,
      encryption: dynamodb.TableEncryptionV2.awsManagedKey(),
    });
    usersTable.addGlobalSecondaryIndex({
      indexName: 'email-index',
      partitionKey: {name: 'email', type: dynamodb.AttributeType.STRING},
      projectionType: dynamodb.ProjectionType.ALL,
      warmThroughput: undefined,
      maxReadRequestUnits: 1000,
      maxWriteRequestUnits: 1000,
    });
    usersTable.addGlobalSecondaryIndex({
      indexName: 'username-index',
      partitionKey: {name: 'username', type: dynamodb.AttributeType.STRING},
      projectionType: dynamodb.ProjectionType.ALL,
      warmThroughput: undefined,
      maxReadRequestUnits: 1000,
      maxWriteRequestUnits: 1000,
    });
    usersTable.addGlobalSecondaryIndex({
      indexName: 'ctech-user-id-index',
      partitionKey: {name: 'ctech_user_id', type: dynamodb.AttributeType.STRING},
      projectionType: dynamodb.ProjectionType.ALL,
      warmThroughput: undefined,
      maxReadRequestUnits: 1000,
      maxWriteRequestUnits: 1000,
    });
    this.tables.set('users', usersTable);

    const organizationsTable = new dynamodb.TableV2(this, `${tablePrefix}_organizations`, {
      tableName: `${tablePrefix}_organizations`,
      partitionKey: {name: 'pk', type: dynamodb.AttributeType.STRING},
      billing: Billing.onDemand({
        maxReadRequestUnits: 1000,
        maxWriteRequestUnits: 1000,
      }),
      removalPolicy,
      pointInTimeRecoverySpecification,
      encryption: dynamodb.TableEncryptionV2.awsManagedKey(),
    });
    this.tables.set('organizations', organizationsTable);

    // Membership is the source of truth for user↔organization access (RBAC, /auth/me, member management).
    const organizationUsersTable = new dynamodb.TableV2(this, `${tablePrefix}_organization_users`, {
      tableName: `${tablePrefix}_organization_users`,
      partitionKey: {name: 'pk', type: dynamodb.AttributeType.STRING},
      sortKey: {name: 'sk', type: dynamodb.AttributeType.STRING},
      billing: Billing.onDemand({
        maxReadRequestUnits: 1000,
        maxWriteRequestUnits: 1000,
      }),
      removalPolicy,
      pointInTimeRecoverySpecification,
      encryption: dynamodb.TableEncryptionV2.awsManagedKey(),
    });
    // Inverted index: partition on the member SK ("USER_{sub}") to list every
    // org a user belongs to (/auth/me, GET /organizations). No attribute
    // duplication — reuses the base pk/sk.
    organizationUsersTable.addGlobalSecondaryIndex({
      indexName: 'user-index',
      partitionKey: {name: 'sk', type: dynamodb.AttributeType.STRING},
      sortKey: {name: 'pk', type: dynamodb.AttributeType.STRING},
      projectionType: dynamodb.ProjectionType.ALL,
      warmThroughput: undefined,
      maxReadRequestUnits: 1000,
      maxWriteRequestUnits: 1000,
    });
    this.tables.set('organization_users', organizationUsersTable);

    // Single-use invitation links. Partition key is the SHA-256 of the opaque
    // token so acceptance is a strongly-consistent GetItem (never a Scan). TTL
    // on `ttl` (epoch seconds) is housekeeping only — expiry is always
    // re-checked in code, since DynamoDB TTL can lag up to 48h.
    const organizationInvitationsTable = new dynamodb.TableV2(this, `${tablePrefix}_organization_invitations`, {
      tableName: `${tablePrefix}_organization_invitations`,
      partitionKey: {name: 'pk', type: dynamodb.AttributeType.STRING},
      billing: Billing.onDemand({
        maxReadRequestUnits: 1000,
        maxWriteRequestUnits: 1000,
      }),
      timeToLiveAttribute: 'ttl',
      removalPolicy,
      pointInTimeRecoverySpecification,
      encryption: dynamodb.TableEncryptionV2.awsManagedKey(),
    });
    // List an org's pending invitations, newest first.
    organizationInvitationsTable.addGlobalSecondaryIndex({
      indexName: 'org-invite-index',
      partitionKey: {name: 'org_pk', type: dynamodb.AttributeType.STRING},
      sortKey: {name: 'created_at', type: dynamodb.AttributeType.STRING},
      projectionType: dynamodb.ProjectionType.ALL,
      warmThroughput: undefined,
      maxReadRequestUnits: 1000,
      maxWriteRequestUnits: 1000,
    });
    this.tables.set('organization_invitations', organizationInvitationsTable);

    // What ctech-billing says about each account, plus the ids of the webhooks
    // already processed.
    //
    // Two row shapes in one table because they share a subject and a lifetime:
    // `USER_{sub}` is the subscription snapshot, `EVENT_{id}` is a delivery
    // already handled, and a second table for a set of ids with a TTL would be a
    // second thing to create, grant and remember.
    //
    // The snapshot is a cache with a durable floor: billing owns the
    // subscription, and this row is what the last read said, so a quota check on
    // the issuance path is a GetItem rather than a call across the network — and
    // so an emission stays decidable while billing is unreachable.
    //
    // No GSI. Every access is by primary key: the snapshot by account, the
    // marker by event id.
    const accountBillingTable = new dynamodb.TableV2(this, `${tablePrefix}_account_billing`, {
      tableName: `${tablePrefix}_account_billing`,
      partitionKey: {name: 'pk', type: dynamodb.AttributeType.STRING},
      billing: Billing.onDemand({
        maxReadRequestUnits: 1000,
        maxWriteRequestUnits: 1000,
      }),
      // Only the EVENT_ rows carry `ttl`. A snapshot has none and must never get
      // one: an account whose row expired would read as "never subscribed" and
      // be refused service it is paying for.
      timeToLiveAttribute: 'ttl',
      removalPolicy,
      pointInTimeRecoverySpecification,
      encryption: dynamodb.TableEncryptionV2.awsManagedKey(),
    });
    this.tables.set('account_billing', accountBillingTable);

    // ============== AUDIT ==============

    const auditLogsTable = new dynamodb.TableV2(this, `${tablePrefix}_audit_logs`, {
      tableName: `${tablePrefix}_audit_logs`,
      partitionKey: {name: 'pk', type: dynamodb.AttributeType.STRING},
      sortKey: {name: 'sk', type: dynamodb.AttributeType.STRING},
      billing: Billing.onDemand({
        maxReadRequestUnits: 1000,
        maxWriteRequestUnits: 1000,
      }),
      removalPolicy,
      pointInTimeRecoverySpecification,
      encryption: dynamodb.TableEncryptionV2.awsManagedKey(),
    });
    auditLogsTable.addGlobalSecondaryIndex({
      indexName: 'org-time-index',
      partitionKey: {name: 'pk', type: dynamodb.AttributeType.STRING},
      sortKey: {name: 'created_at', type: dynamodb.AttributeType.STRING},
      projectionType: dynamodb.ProjectionType.ALL,
      warmThroughput: undefined,
      maxReadRequestUnits: 1000,
      maxWriteRequestUnits: 1000,
    });
    auditLogsTable.addGlobalSecondaryIndex({
      indexName: 'user-id-index',
      partitionKey: {name: 'user_id', type: dynamodb.AttributeType.STRING},
      sortKey: {name: 'created_at', type: dynamodb.AttributeType.STRING},
      projectionType: dynamodb.ProjectionType.ALL,
      warmThroughput: undefined,
      maxReadRequestUnits: 1000,
      maxWriteRequestUnits: 1000,
    });
    this.tables.set('audit_logs', auditLogsTable);

    // ============== ORGANIZATION RESOURCES ==============

    const certificatesTable = new dynamodb.TableV2(this, `${tablePrefix}_organization_certificates`, {
      tableName: `${tablePrefix}_organization_certificates`,
      partitionKey: {name: 'pk', type: dynamodb.AttributeType.STRING},
      sortKey: {name: 'sk', type: dynamodb.AttributeType.STRING},
      billing: Billing.onDemand({
        maxReadRequestUnits: 1000,
        maxWriteRequestUnits: 1000,
      }),
      removalPolicy,
      pointInTimeRecoverySpecification,
      encryption: dynamodb.TableEncryptionV2.awsManagedKey(),
    });
    this.tables.set('certificates', certificatesTable);

    const productsTable = new dynamodb.TableV2(this, `${tablePrefix}_organization_products`, {
      tableName: `${tablePrefix}_organization_products`,
      partitionKey: {name: 'pk', type: dynamodb.AttributeType.STRING},
      sortKey: {name: 'sk', type: dynamodb.AttributeType.STRING},
      billing: Billing.onDemand({
        maxReadRequestUnits: 1000,
        maxWriteRequestUnits: 1000,
      }),
      removalPolicy,
      pointInTimeRecoverySpecification,
      encryption: dynamodb.TableEncryptionV2.awsManagedKey(),
    });
    productsTable.addGlobalSecondaryIndex({
      indexName: 'description-index',
      partitionKey: {name: 'pk', type: dynamodb.AttributeType.STRING},
      sortKey: {name: 'description', type: dynamodb.AttributeType.STRING},
      projectionType: dynamodb.ProjectionType.ALL,
      warmThroughput: undefined,
      maxReadRequestUnits: 1000,
      maxWriteRequestUnits: 1000,
    });
    productsTable.addGlobalSecondaryIndex({
      indexName: 'code-index',
      partitionKey: {name: 'pk', type: dynamodb.AttributeType.STRING},
      sortKey: {name: 'code', type: dynamodb.AttributeType.STRING},
      projectionType: dynamodb.ProjectionType.ALL,
      warmThroughput: undefined,
      maxReadRequestUnits: 1000,
      maxWriteRequestUnits: 1000,
    });
    this.tables.set('products', productsTable);

    const vehiclesTable = new dynamodb.TableV2(this, `${tablePrefix}_organization_vehicles`, {
      tableName: `${tablePrefix}_organization_vehicles`,
      partitionKey: {name: 'pk', type: dynamodb.AttributeType.STRING},
      sortKey: {name: 'sk', type: dynamodb.AttributeType.STRING},
      billing: Billing.onDemand({
        maxReadRequestUnits: 1000,
        maxWriteRequestUnits: 1000,
      }),
      removalPolicy,
      pointInTimeRecoverySpecification,
      encryption: dynamodb.TableEncryptionV2.awsManagedKey(),
    });
    vehiclesTable.addGlobalSecondaryIndex({
      indexName: 'plate-index',
      partitionKey: {name: 'pk', type: dynamodb.AttributeType.STRING},
      sortKey: {name: 'plate', type: dynamodb.AttributeType.STRING},
      projectionType: dynamodb.ProjectionType.ALL,
      warmThroughput: undefined,
      maxReadRequestUnits: 1000,
      maxWriteRequestUnits: 1000,
    });
    vehiclesTable.addGlobalSecondaryIndex({
      indexName: 'role-index',
      partitionKey: {name: 'pk', type: dynamodb.AttributeType.STRING},
      sortKey: {name: 'role', type: dynamodb.AttributeType.STRING},
      projectionType: dynamodb.ProjectionType.ALL,
      warmThroughput: undefined,
      maxReadRequestUnits: 1000,
      maxWriteRequestUnits: 1000,
    });
    this.tables.set('vehicles', vehiclesTable);

    const personsTable = new dynamodb.TableV2(this, `${tablePrefix}_organization_persons`, {
      tableName: `${tablePrefix}_organization_persons`,
      partitionKey: {name: 'pk', type: dynamodb.AttributeType.STRING},
      sortKey: {name: 'sk', type: dynamodb.AttributeType.STRING},
      billing: Billing.onDemand({
        maxReadRequestUnits: 1000,
        maxWriteRequestUnits: 1000,
      }),
      removalPolicy,
      pointInTimeRecoverySpecification,
      encryption: dynamodb.TableEncryptionV2.awsManagedKey(),
    });
    personsTable.addGlobalSecondaryIndex({
      indexName: 'org-name-index',
      partitionKey: {name: 'pk', type: dynamodb.AttributeType.STRING},
      sortKey: {name: 'name', type: dynamodb.AttributeType.STRING},
      projectionType: dynamodb.ProjectionType.ALL,
      warmThroughput: undefined,
      maxReadRequestUnits: 1000,
      maxWriteRequestUnits: 1000,
    });
    this.tables.set('persons', personsTable);

    const servicesTable = new dynamodb.TableV2(this, `${tablePrefix}_organization_services`, {
      tableName: `${tablePrefix}_organization_services`,
      partitionKey: {name: 'pk', type: dynamodb.AttributeType.STRING},
      sortKey: {name: 'sk', type: dynamodb.AttributeType.STRING},
      billing: Billing.onDemand({
        maxReadRequestUnits: 1000,
        maxWriteRequestUnits: 1000,
      }),
      removalPolicy,
      pointInTimeRecoverySpecification,
      encryption: dynamodb.TableEncryptionV2.awsManagedKey(),
    });
    servicesTable.addGlobalSecondaryIndex({
      indexName: 'description-index',
      partitionKey: {name: 'pk', type: dynamodb.AttributeType.STRING},
      sortKey: {name: 'description', type: dynamodb.AttributeType.STRING},
      projectionType: dynamodb.ProjectionType.ALL,
      warmThroughput: undefined,
      maxReadRequestUnits: 1000,
      maxWriteRequestUnits: 1000,
    });
    servicesTable.addGlobalSecondaryIndex({
      indexName: 'code-index',
      partitionKey: {name: 'pk', type: dynamodb.AttributeType.STRING},
      sortKey: {name: 'code', type: dynamodb.AttributeType.STRING},
      projectionType: dynamodb.ProjectionType.ALL,
      warmThroughput: undefined,
      maxReadRequestUnits: 1000,
      maxWriteRequestUnits: 1000,
    });
    this.tables.set('services', servicesTable);

    // ============== REUSABLE REGISTRY TABLES ==============
    // Decisões que hoje são redigitadas a cada emissão viram entidade nomeada.
    // Todas compartilham a mesma forma (pk/sk + name-index) — ver getOrgEntityTable.

    this.tables.set('tax_profiles', getOrgEntityTable(
      this, removalPolicy, pointInTimeRecoverySpecification, tablePrefix, 'tax_profiles'));
    this.tables.set('operations', getOrgEntityTable(
      this, removalPolicy, pointInTimeRecoverySpecification, tablePrefix, 'operations'));
    this.tables.set('payment_terms', getOrgEntityTable(
      this, removalPolicy, pointInTimeRecoverySpecification, tablePrefix, 'payment_terms'));
    this.tables.set('vehicle_sets', getOrgEntityTable(
      this, removalPolicy, pointInTimeRecoverySpecification, tablePrefix, 'vehicle_sets'));

    // ============== CONFIGURATION TABLES ==============

    const nfeConfigTable = getDfeConfigTable(this, removalPolicy, pointInTimeRecoverySpecification, tablePrefix, 'nfe_configs');
    this.tables.set('nfe_configs', nfeConfigTable);

    const nfceConfigTable = getDfeConfigTable(this, removalPolicy, pointInTimeRecoverySpecification, tablePrefix, 'nfce_configs');
    this.tables.set('nfce_configs', nfceConfigTable);

    const cteConfigTable = getDfeConfigTable(this, removalPolicy, pointInTimeRecoverySpecification, tablePrefix, 'cte_configs');
    this.tables.set('cte_configs', cteConfigTable);

    const mdfeConfigTable = getDfeConfigTable(this, removalPolicy, pointInTimeRecoverySpecification, tablePrefix, 'mdfe_configs');
    this.tables.set('mdfe_configs', mdfeConfigTable);

    const nfseConfigTable = getDfeConfigTable(this, removalPolicy, pointInTimeRecoverySpecification, tablePrefix, 'nfse_configs');
    this.tables.set('nfse_configs', nfseConfigTable);

    // ============== DOCUMENT & EVENT TABLES ==============

    const nfeEventsTable = getEventsTable(this, removalPolicy, pointInTimeRecoverySpecification, tablePrefix, 'nfe_events');
    this.tables.set('nfe_events', nfeEventsTable);

    const nfceEventsTable = getEventsTable(this, removalPolicy, pointInTimeRecoverySpecification, tablePrefix, 'nfce_events');
    this.tables.set('nfce_events', nfceEventsTable);

    const cteEventsTable = getEventsTable(this, removalPolicy, pointInTimeRecoverySpecification, tablePrefix, 'cte_events');
    this.tables.set('cte_events', cteEventsTable);

    const mdfeEventsTable = getEventsTable(this, removalPolicy, pointInTimeRecoverySpecification, tablePrefix, 'mdfe_events');
    this.tables.set('mdfe_events', mdfeEventsTable);

    const nfseEventsTable = getEventsTable(this, removalPolicy, pointInTimeRecoverySpecification, tablePrefix, 'nfse_events');
    this.tables.set('nfse_events', nfseEventsTable);

    const nfesTable = getDfeTable(this, removalPolicy, pointInTimeRecoverySpecification, tablePrefix, 'nfes');
    this.tables.set('nfes', nfesTable);

    const nfcesTable = getDfeTable(this, removalPolicy, pointInTimeRecoverySpecification, tablePrefix, 'nfces');
    this.tables.set('nfces', nfcesTable);

    const ctesTable = getDfeTable(this, removalPolicy, pointInTimeRecoverySpecification, tablePrefix, 'ctes');
    this.tables.set('ctes', ctesTable);

    const mdfesTable = getDfeTable(this, removalPolicy, pointInTimeRecoverySpecification, tablePrefix, 'mdfes');
    this.tables.set('mdfes', mdfesTable);

    // nfses reutiliza getDfeTable (number-index-v2 + dfe-index) e acrescenta
    // access-key-index: a SK é o id_dps, porque a chave de acesso de 50
    // dígitos só existe depois da resposta do fisco — ver
    // docs/specs/2026-08-04-nfse-design.md §3.4.
    const nfsesTable = getDfeTable(this, removalPolicy, pointInTimeRecoverySpecification, tablePrefix, 'nfses');
    nfsesTable.addGlobalSecondaryIndex({
      indexName: 'access-key-index',
      partitionKey: {name: 'pk', type: dynamodb.AttributeType.STRING},
      sortKey: {name: 'access_key', type: dynamodb.AttributeType.STRING},
      projectionType: dynamodb.ProjectionType.ALL,
      warmThroughput: undefined,
      maxReadRequestUnits: 1000,
      maxWriteRequestUnits: 1000,
    });
    this.tables.set('nfses', nfsesTable);

    // ============== DISTRIBUTION TABLES ==============

    const nfeDistributionsTable = getDistributionTable(this, removalPolicy, pointInTimeRecoverySpecification, tablePrefix, 'nfe_distributions');
    this.tables.set('nfe_distributions', nfeDistributionsTable);

    const cteDistributionsTable = getDistributionTable(this, removalPolicy, pointInTimeRecoverySpecification, tablePrefix, 'cte_distributions');
    this.tables.set('cte_distributions', cteDistributionsTable);

    const mdfeDistributionsTable = getDistributionTable(this, removalPolicy, pointInTimeRecoverySpecification, tablePrefix, 'mdfe_distributions');
    this.tables.set('mdfe_distributions', mdfeDistributionsTable);

    const nfseDistributionsTable = getDistributionTable(this, removalPolicy, pointInTimeRecoverySpecification, tablePrefix, 'nfse_distributions');
    this.tables.set('nfse_distributions', nfseDistributionsTable);

    // Transactional command outbox. New images are streamed to the
    // outbox-publisher Lambda; published rows expire after 30 days.
    const workerOutboxTable = new dynamodb.TableV2(this, `${tablePrefix}_worker_outbox`, {
      tableName: `${tablePrefix}_worker_outbox`,
      partitionKey: {name: 'pk', type: dynamodb.AttributeType.STRING},
      sortKey: {name: 'sk', type: dynamodb.AttributeType.STRING},
      billing: Billing.onDemand({
        maxReadRequestUnits: 1000,
        maxWriteRequestUnits: 1000,
      }),
      dynamoStream: dynamodb.StreamViewType.NEW_IMAGE,
      timeToLiveAttribute: 'ttl',
      removalPolicy,
      pointInTimeRecoverySpecification,
      encryption: dynamodb.TableEncryptionV2.awsManagedKey(),
    });
    this.tables.set('worker_outbox', workerOutboxTable);

    // ============== OUTPUTS ==============

    new cdk.CfnOutput(this, 'UsersTableName', {
      value: usersTable.tableName,
      exportName: `${id}-users-table`,
    });

    new cdk.CfnOutput(this, 'OrganizationsTableName', {
      value: organizationsTable.tableName,
      exportName: `${id}-organizations-table`,
    });
  }
}
