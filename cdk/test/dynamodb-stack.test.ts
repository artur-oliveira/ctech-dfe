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
