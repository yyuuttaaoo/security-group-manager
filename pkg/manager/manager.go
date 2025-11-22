package manager

import (
	"fmt"
	"log/slog"
	"strings"

	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	ecs20140526 "github.com/alibabacloud-go/ecs-20140526/v7/client"
	swas "github.com/alibabacloud-go/swas-open-20200601/v3/client"
	util "github.com/alibabacloud-go/tea-utils/v2/service"
	"github.com/alibabacloud-go/tea/tea"
	credential "github.com/aliyun/credentials-go/credentials"
)

type SecurityGroupManager struct {
	Client   *ecs20140526.Client
	RegionId string
}

// NewSecurityGroupManager 创建SecurityGroupManager实例
func NewSecurityGroupManager(regionId string) (*SecurityGroupManager, error) {
	// 工程代码建议使用更安全的无AK方式，凭据配置方式请参见：https://help.aliyun.com/document_detail/378661.html。
	credential, err := credential.NewCredential(nil)
	if err != nil {
		return nil, err
	}

	config := &openapi.Config{
		Credential: credential,
	}
	// Endpoint 请参考 https://api.aliyun.com/product/Ecs
	config.Endpoint = tea.String(fmt.Sprintf("ecs.%s.aliyuncs.com", regionId))

	client, err := ecs20140526.NewClient(config)
	if err != nil {
		return nil, err
	}

	return &SecurityGroupManager{
		Client:   client,
		RegionId: regionId,
	}, nil
}

// GetSecurityGroups 获取安全组列表
func (sgm *SecurityGroupManager) GetSecurityGroups() (*ecs20140526.DescribeSecurityGroupsResponseBody, error) {
	tag0 := &ecs20140526.DescribeSecurityGroupsRequestTag{
		// String, 可选, 安全组的标签键。  > 为提高兼容性，建议您尽量使用Tag.N.Key参数。
		Key: tea.String("auto-manage"),
		// String, 可选, 安全组的标签值。N的取值范围：1~20。
		Value: tea.String("true"),
	}
	describeSecurityGroupsRequest := &ecs20140526.DescribeSecurityGroupsRequest{
		RegionId: tea.String(sgm.RegionId),
		// Array, 可选
		Tag: []*ecs20140526.DescribeSecurityGroupsRequestTag{tag0},
	}
	runtime := &util.RuntimeOptions{}

	resp, err := sgm.Client.DescribeSecurityGroupsWithOptions(describeSecurityGroupsRequest, runtime)
	if err != nil {
		return nil, err
	}
	return resp.GetBody(), nil
}

// GetSecurityGroupRules 获取安全组规则
func (sgm *SecurityGroupManager) GetSecurityGroupRules(securityGroupId string) (*ecs20140526.DescribeSecurityGroupAttributeResponseBody, error) {
	describeSecurityGroupAttributeRequest := &ecs20140526.DescribeSecurityGroupAttributeRequest{
		RegionId:        tea.String(sgm.RegionId),
		SecurityGroupId: tea.String(securityGroupId),
	}
	runtime := &util.RuntimeOptions{}

	resp, err := sgm.Client.DescribeSecurityGroupAttributeWithOptions(describeSecurityGroupAttributeRequest, runtime)
	if err != nil {
		return nil, err
	}
	return resp.GetBody(), nil
}

func (sgm *SecurityGroupManager) UpdateSecurityGroupRuleSourceCidrIp(securityGroupId, securityGroupRuleId, sourceCidrIp string) error {
	modifySecurityGroupRuleRequest := &ecs20140526.ModifySecurityGroupRuleRequest{
		RegionId:            tea.String(sgm.RegionId),
		SecurityGroupId:     tea.String(securityGroupId),
		SecurityGroupRuleId: tea.String(securityGroupRuleId),
		SourceCidrIp:        tea.String(sourceCidrIp),
	}
	runtime := &util.RuntimeOptions{}

	_, err := sgm.Client.ModifySecurityGroupRuleWithOptions(modifySecurityGroupRuleRequest, runtime)
	if err != nil {
		return err
	}
	return nil
}

// PrintRuleSummary logs summary using the provided logger
func PrintRuleSummary(logger *slog.Logger, rule *ecs20140526.DescribeSecurityGroupAttributeResponseBodyPermissionsPermission, overrideSrcCidrIp *string) {
	status := "UNCHANGED"
	srcIp := tea.StringValue(rule.SourceCidrIp)
	if overrideSrcCidrIp != nil {
		status = "UPDATED"
		srcIp = tea.StringValue(overrideSrcCidrIp)
	}

	logger.Info("Security Group Rule",
		"status", status,
		"ruleId", tea.StringValue(rule.SecurityGroupRuleId),
		"sourceCidrIp", srcIp,
		"portRange", tea.StringValue(rule.PortRange),
		"description", tea.StringValue(rule.Description),
	)
}

type SWASManager struct {
	Client   *swas.Client
	RegionId string
}

func NewSWASManager(regionId string) (*SWASManager, error) {
	credential, err := credential.NewCredential(nil)
	if err != nil {
		return nil, err
	}

	config := &openapi.Config{
		Credential: credential,
	}
	config.Endpoint = tea.String(fmt.Sprintf("swas.%s.aliyuncs.com", regionId))

	client, err := swas.NewClient(config)
	if err != nil {
		return nil, err
	}

	return &SWASManager{
		Client:   client,
		RegionId: regionId,
	}, nil
}

func (sm *SWASManager) GetInstances() ([]*swas.ListInstancesResponseBodyInstances, error) {
	tag0 := &swas.ListInstancesRequestTag{
		Key:   tea.String("auto-manage"),
		Value: tea.String("true"),
	}
	request := &swas.ListInstancesRequest{
		RegionId: tea.String(sm.RegionId),
		Tag:      []*swas.ListInstancesRequestTag{tag0},
	}
	runtime := &util.RuntimeOptions{}

	resp, err := sm.Client.ListInstancesWithOptions(request, runtime)
	if err != nil {
		return nil, err
	}

	return resp.Body.Instances, nil
}

func (sm *SWASManager) GetFirewallRules(instanceId string) ([]*swas.ListFirewallRulesResponseBodyFirewallRules, error) {
	request := &swas.ListFirewallRulesRequest{
		RegionId:   tea.String(sm.RegionId),
		InstanceId: tea.String(instanceId),
	}
	runtime := &util.RuntimeOptions{}

	resp, err := sm.Client.ListFirewallRulesWithOptions(request, runtime)
	if err != nil {
		return nil, err
	}
	return resp.Body.FirewallRules, nil
}

func (sm *SWASManager) ModifyFirewallRule(instanceId, ruleId, protocol, port, sourceIp, remark string) error {
	request := &swas.ModifyFirewallRuleRequest{
		RegionId:     tea.String(sm.RegionId),
		InstanceId:   tea.String(instanceId),
		RuleId:       tea.String(ruleId),
		RuleProtocol: tea.String(protocol),
		Port:         tea.String(port),
		SourceCidrIp: tea.String(sourceIp),
		Remark:       tea.String(remark),
	}
	runtime := &util.RuntimeOptions{}

	_, err := sm.Client.ModifyFirewallRuleWithOptions(request, runtime)
	return err
}

func PrintSwasRuleSummary(logger *slog.Logger, rule *swas.ListFirewallRulesResponseBodyFirewallRules, overrideSrcCidrIp *string) {
	status := "UNCHANGED"
	srcIp := tea.StringValue(rule.SourceCidrIp)
	if overrideSrcCidrIp != nil {
		status = "UPDATED"
		srcIp = tea.StringValue(overrideSrcCidrIp)
	}

	logger.Info("SWAS Firewall Rule",
		"status", status,
		"ruleId", tea.StringValue(rule.RuleId),
		"sourceCidrIp", srcIp,
		"port", tea.StringValue(rule.Port),
		"protocol", tea.StringValue(rule.RuleProtocol),
		"remark", tea.StringValue(rule.Remark),
	)
}

func ProcessRegion(regionId string, currentIP string, logger *slog.Logger) error {
	sgm, err := NewSecurityGroupManager(regionId)
	if err != nil {
		return err
	}

	securityGroupsResp, err := sgm.GetSecurityGroups()
	if err != nil {
		return err
	}

	securityGroups := securityGroupsResp.GetSecurityGroups().GetSecurityGroup()
	for _, securityGroup := range securityGroups {
		securityGroupRules, err := sgm.GetSecurityGroupRules(*securityGroup.GetSecurityGroupId())
		if err != nil {
			return err
		}

		for _, permissionRule := range securityGroupRules.GetPermissions().GetPermission() {
			if strings.HasPrefix(tea.StringValue(permissionRule.Description), "auto-manage-") {
				if tea.StringValue(permissionRule.SourceCidrIp) != currentIP {
					logger.Info("Updating permission rule", "region", regionId, "securityGroupId", *securityGroup.GetSecurityGroupId())
					errUpdate := sgm.UpdateSecurityGroupRuleSourceCidrIp(*securityGroup.GetSecurityGroupId(), *permissionRule.GetSecurityGroupRuleId(), currentIP)
					if errUpdate != nil {
						return errUpdate
					}
					PrintRuleSummary(logger, permissionRule, &currentIP)
				} else {
					PrintRuleSummary(logger, permissionRule, nil)
				}
			}
		}
	}

	// SWAS Logic
	swasMgr, err := NewSWASManager(regionId)
	if err != nil {
		// Log error but continue, as some regions might not support SWAS or other issues
		logger.Warn("Failed to init SWAS manager", "region", regionId, "error", err)
	} else {
		swasInstances, err := swasMgr.GetInstances()
		if err != nil {
			logger.Warn("Failed to list SWAS instances", "region", regionId, "error", err)
		} else {
			for _, instance := range swasInstances {
				logger.Info("Checking SWAS Instance", "name", tea.StringValue(instance.InstanceName), "id", tea.StringValue(instance.InstanceId))
				rules, err := swasMgr.GetFirewallRules(*instance.InstanceId)
				if err != nil {
					return err
				}

				for _, rule := range rules {
					if strings.HasPrefix(tea.StringValue(rule.Remark), "auto-manage-") {
						if tea.StringValue(rule.SourceCidrIp) != currentIP {
							logger.Info("Updating SWAS firewall rule", "region", regionId, "instanceId", tea.StringValue(instance.InstanceId))
							errUpdate := swasMgr.ModifyFirewallRule(
								*instance.InstanceId,
								*rule.RuleId,
								*rule.RuleProtocol,
								*rule.Port,
								currentIP,
								*rule.Remark,
							)
							if errUpdate != nil {
								return errUpdate
							}
							PrintSwasRuleSummary(logger, rule, &currentIP)
						} else {
							PrintSwasRuleSummary(logger, rule, nil)
						}
					}
				}
			}
		}
	}

	return nil
}
