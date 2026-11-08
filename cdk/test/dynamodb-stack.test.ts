import * as cdk from 'aws-cdk-lib';
import {Match, Template} from 'aws-cdk-lib/assertions';
import {DynamoDBStack} from '../lib/dynamodb-stack';

const synth = () => {
    const app = new cdk.App();
    const stack = new DynamoDBStack(app, 'TestDynamoDBStack', {
        tablePrefix: 'dev_dfe',
        environment: 'dev',
    });
    return Template.fromStack(stack);
};

describe('DynamoDBStack — tabelas NFS-e', () => {
    test('cria as quatro tabelas novas', () => {
        const template = synth();
        for (const name of [
            'dev_dfe_organization_services',
            'dev_dfe_organization_nfse_configs',
            'dev_dfe_nfses',
            'dev_dfe_nfse_events',
        ]) {
            template.hasResourceProperties('AWS::DynamoDB::GlobalTable', {TableName: name});
        }
    });

    test('organization_services tem os GSIs code-index e description-index', () => {
        const template = synth();
        template.hasResourceProperties('AWS::DynamoDB::GlobalTable', {
            TableName: 'dev_dfe_organization_services',
            GlobalSecondaryIndexes: Match.arrayWith([
                Match.objectLike({IndexName: 'description-index'}),
                Match.objectLike({IndexName: 'code-index'}),
            ]),
        });
    });

    test('nfses tem o GSI access-key-index', () => {
        const template = synth();
        template.hasResourceProperties('AWS::DynamoDB::GlobalTable', {
            TableName: 'dev_dfe_nfses',
            GlobalSecondaryIndexes: Match.arrayWith([
                Match.objectLike({IndexName: 'access-key-index'}),
            ]),
        });
    });

    test('nfse_events tem PK/SK genéricos (pk/sk) e o GSI org-event-key-index', () => {
        const template = synth();
        template.hasResourceProperties('AWS::DynamoDB::GlobalTable', {
            TableName: 'dev_dfe_nfse_events',
            KeySchema: Match.arrayWith([
                Match.objectLike({AttributeName: 'pk', KeyType: 'HASH'}),
                Match.objectLike({AttributeName: 'sk', KeyType: 'RANGE'}),
            ]),
            GlobalSecondaryIndexes: Match.arrayWith([
                Match.objectLike({IndexName: 'org-event-key-index'}),
            ]),
        });
    });

    test('nenhuma tabela NFS-e declara stream — o outbox usa worker_outbox', () => {
        const template = synth();
        const tables = template.findResources('AWS::DynamoDB::GlobalTable');
        for (const res of Object.values(tables) as any[]) {
            const name: string = res.Properties.TableName;
            if (name.includes('nfse')) {
                expect(res.Properties.StreamSpecification).toBeUndefined();
            }
        }
    });
});

describe('DynamoDBStack — organizations por workspace', () => {
    test('organizations tem o GSI organization-index (organization_id, created_at), só chaves', () => {
        const template = synth();
        template.hasResourceProperties('AWS::DynamoDB::GlobalTable', {
            TableName: 'dev_dfe_organizations',
            GlobalSecondaryIndexes: Match.arrayWith([
                Match.objectLike({
                    IndexName: 'organization-index',
                    KeySchema: [
                        {AttributeName: 'organization_id', KeyType: 'HASH'},
                        {AttributeName: 'created_at', KeyType: 'RANGE'},
                    ],
                    Projection: {ProjectionType: 'KEYS_ONLY'},
                }),
            ]),
        });
    });
});

describe('DynamoDBStack — níveis de cobrança pendentes', () => {
    test('account_billing tem o GSI esparso level-dirty-index', () => {
        const template = synth();
        template.hasResourceProperties('AWS::DynamoDB::GlobalTable', {
            TableName: 'dev_dfe_account_billing',
            GlobalSecondaryIndexes: Match.arrayWith([
                Match.objectLike({
                    IndexName: 'level-dirty-index',
                    KeySchema: [
                        {AttributeName: 'dirty_shard', KeyType: 'HASH'},
                        {AttributeName: 'changed_at', KeyType: 'RANGE'},
                    ],
                }),
            ]),
        });
    });
});

describe('DynamoDBStack — exclusão de conta', () => {
    test('erasure_state: só pk, com TTL em ttl', () => {
        synth().hasResourceProperties('AWS::DynamoDB::GlobalTable', {
            TableName: 'dev_dfe_erasure_state',
            KeySchema: [{AttributeName: 'pk', KeyType: 'HASH'}],
            TimeToLiveSpecification: {AttributeName: 'ttl', Enabled: true},
        });
    });
});

describe('DynamoDBStack — idempotência', () => {
    test('idempotency: pk+sk, com TTL em ttl', () => {
        synth().hasResourceProperties('AWS::DynamoDB::GlobalTable', {
            TableName: 'dev_dfe_idempotency',
            KeySchema: [{AttributeName: 'pk', KeyType: 'HASH'}, {AttributeName: 'sk', KeyType: 'RANGE'}],
            TimeToLiveSpecification: {AttributeName: 'ttl', Enabled: true},
        });
    });
});

describe('DynamoDBStack — sweep-index', () => {
    test('configs despachadas e certificados têm o GSI sweep-index (KEYS_ONLY)', () => {
        const template = synth();
        for (const name of [
            'dev_dfe_organization_nfe_configs',
            'dev_dfe_organization_cte_configs',
            'dev_dfe_organization_mdfe_configs',
            'dev_dfe_organization_nfse_configs',
            'dev_dfe_organization_certificates',
        ]) {
            template.hasResourceProperties('AWS::DynamoDB::GlobalTable', {
                TableName: name,
                GlobalSecondaryIndexes: Match.arrayWith([
                    Match.objectLike({
                        IndexName: 'sweep-index',
                        KeySchema: [{AttributeName: 'sweep', KeyType: 'HASH'}],
                        Projection: {ProjectionType: 'KEYS_ONLY'},
                    }),
                ]),
            });
        }
    });
});

describe('DynamoDBStack — contingency-index', () => {
    test('as quatro tabelas de documento têm o GSI esparso contingency-index (KEYS_ONLY, ordenado por prazo)', () => {
        const template = synth();
        for (const name of ['dev_dfe_nfes', 'dev_dfe_nfces', 'dev_dfe_ctes', 'dev_dfe_mdfes']) {
            template.hasResourceProperties('AWS::DynamoDB::GlobalTable', {
                TableName: name,
                GlobalSecondaryIndexes: Match.arrayWith([
                    Match.objectLike({
                        IndexName: 'contingency-index',
                        KeySchema: [
                            {AttributeName: 'contingency_sweep', KeyType: 'HASH'},
                            {AttributeName: 'contingency_deadline', KeyType: 'RANGE'},
                        ],
                        Projection: {ProjectionType: 'KEYS_ONLY'},
                    }),
                ]),
            });
        }
    });
});

// KMS-encrypted tables bill a KMS request per read/write; the default AWS-owned key is free.
// Changing this on an existing GlobalTable is an in-place update (SSESpecification: no interruption).
describe('DynamoDBStack — encryption', () => {
    test('no table uses a KMS key (AWS-owned key only)', () => {
        const tables: any[] = Object.values(synth().findResources('AWS::DynamoDB::GlobalTable'));
        expect(tables.length).toBeGreaterThan(20);
        for (const t of tables) {
            expect(t.Properties.SSESpecification).toEqual({SSEEnabled: false});
        }
    });
});

describe('DynamoDBStack - CIOT', () => {
    test('ciots tem code-index, mdfe-index e TTL expires_at', () => {
        synth().hasResourceProperties('AWS::DynamoDB::GlobalTable', {
            TableName: 'dev_dfe_ciots',
            TimeToLiveSpecification: {AttributeName: 'expires_at', Enabled: true},
            GlobalSecondaryIndexes: Match.arrayWith([
                Match.objectLike({IndexName: 'code-index'}),
                Match.objectLike({IndexName: 'mdfe-index'}),
            ]),
        });
    });

    test('ciots tem o alert-index esparso, KEYS_ONLY', () => {
        synth().hasResourceProperties('AWS::DynamoDB::GlobalTable', {
            TableName: 'dev_dfe_ciots',
            GlobalSecondaryIndexes: Match.arrayWith([
                Match.objectLike({
                    IndexName: 'alert-index',
                    Projection: {ProjectionType: 'KEYS_ONLY'},
                    KeySchema: [
                        {AttributeName: 'alert_sweep', KeyType: 'HASH'},
                        {AttributeName: 'alert_at', KeyType: 'RANGE'},
                    ],
                }),
            ]),
        });
    });

    test('ciot_audit e pk/sk sem indices e sem TTL', () => {
        const t = synth();
        t.hasResourceProperties('AWS::DynamoDB::GlobalTable', {
            TableName: 'dev_dfe_ciot_audit',
            KeySchema: Match.arrayWith([
                Match.objectLike({AttributeName: 'pk', KeyType: 'HASH'}),
                Match.objectLike({AttributeName: 'sk', KeyType: 'RANGE'}),
            ]),
        });
        const res = Object.values(t.findResources('AWS::DynamoDB::GlobalTable')) as any[];
        const audit = res.find((r) => r.Properties.TableName === 'dev_dfe_ciot_audit');
        expect(audit.Properties.TimeToLiveSpecification).toBeUndefined();
        expect(audit.Properties.GlobalSecondaryIndexes).toBeUndefined();
    });
});

describe('DynamoDBStack — auto-contingency-index', () => {
    test('as quatro configuracoes fiscais tem o GSI esparso auto-contingency-index (KEYS_ONLY, auto_sweep)', () => {
        const template = synth();
        for (const name of [
            'dev_dfe_organization_nfe_configs',
            'dev_dfe_organization_nfce_configs',
            'dev_dfe_organization_cte_configs',
            'dev_dfe_organization_mdfe_configs',
        ]) {
            template.hasResourceProperties('AWS::DynamoDB::GlobalTable', {
                TableName: name,
                GlobalSecondaryIndexes: Match.arrayWith([
                    Match.objectLike({
                        IndexName: 'auto-contingency-index',
                        KeySchema: [{AttributeName: 'auto_sweep', KeyType: 'HASH'}],
                        Projection: {ProjectionType: 'KEYS_ONLY'},
                    }),
                ]),
            });
        }
    });
});
