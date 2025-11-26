use anchor_lang::prelude::*;

declare_id!("ECL8142j2YQAvs9R9geSsRnkVH2wLEi7soJCRyJ74cfL");

#[program]
pub mod data_storage {
    use super::*;

    // simulate
    pub fn get_reserves(_ctx: Context<GetReserves>) -> Result<UpdateReserves> {
        Ok(UpdateReserves {
            total_minted: 100,
            total_reserve: 200,
        })
    }

    // simulate
    pub fn get_multiple_reserves(
        _ctx: Context<GetMultipleReserves>,
    ) -> Result<Vec<UpdateReserves>> {
        let reserves = vec![
            UpdateReserves {
                total_minted: 100,
                total_reserve: 200,
            },
            UpdateReserves {
                total_minted: 300,
                total_reserve: 400,
            },
        ];

        Ok(reserves)
    }

    // simulate
    pub fn get_tuple_reserves(_ctx: Context<GetTupleReserves>) -> Result<(u64, u64)> {
        Ok((100, 200))
    }

    pub fn initialize_data_account(ctx: Context<Initialize>, input: UserData) -> Result<()> {
        ctx.accounts.data_account.sender = ctx.accounts.user.key().to_string();
        ctx.accounts.data_account.key = input.key;
        ctx.accounts.data_account.value = input.value;
        Ok(())
    }

    // no event
    pub fn update_key_value_data(
        ctx: Context<UpdateData>,
        key: String,
        value: String,
    ) -> Result<()> {
        let acc = &mut ctx.accounts.data_account;

        acc.sender = ctx.accounts.user.key().to_string();
        acc.key = key.clone();
        acc.value = value.clone();

        let user_data = UserData {
            key: key.clone(),
            value: value.clone(),
        };

        emit!(DynamicEvent {
            key: key,
            user_data: user_data,
            sender: ctx.accounts.user.key().to_string(),
            metadata: vec![1, 2, 3],
            metadata_array: vec![],
        });

        Ok(())
    }

    pub fn update_user_data(ctx: Context<UpdateData>, input: UserData) -> Result<()> {
        let acc = &mut ctx.accounts.data_account;

        acc.sender = ctx.accounts.user.key().to_string();
        acc.key = input.key.clone();
        acc.value = input.value.clone();

        let user_data_cloned = input.clone();

        emit!(DynamicEvent {
            key: input.key,
            user_data: user_data_cloned,
            sender: ctx.accounts.user.key().to_string(),
            metadata: vec![1, 2, 3],
            metadata_array: vec![],
        });

        Ok(())
    }

    pub fn log_access(ctx: Context<LogAccess>, message: String) -> Result<()> {
        emit!(AccessLogged {
            caller: ctx.accounts.user.key(),
            message,
        });
        Ok(())
    }

    pub fn on_report(ctx: Context<OnReport>, _metadata: Vec<u8>, payload: Vec<u8>) -> Result<()> {
        // decode payload into UserData
        let mut bytes: &[u8] = &payload;
        let user = UserData::deserialize(&mut bytes)?; // requires AnchorDeserialize on UserData

        // update mapping-equivalent: this user's PDA
        let acc = &mut ctx.accounts.data_account;
        acc.sender = ctx.accounts.user.key().to_string();
        acc.key = user.key.clone();
        acc.value = user.value.clone();

        let user_cloned = user.clone();

        // emit event
        emit!(DynamicEvent {
            sender: ctx.accounts.user.key().to_string(),
            key: user.key,
            user_data: user_cloned,
            metadata: vec![1, 2, 3],
            metadata_array: vec![],
        });

        Ok(())
    }

    pub fn handle_forwarder_report(
        _ctx: Context<HandleForwarderReport>,
        _report: ForwarderReport,
    ) -> Result<()> {
        // TODO: implement forwarding logic here
        Ok(())
    }
}

// read data from here
#[account]
pub struct DataAccount {
    pub sender: String,
    pub key: String,
    pub value: String,
}

#[derive(Accounts)]
pub struct Initialize<'info> {
    #[account(
        init,
        payer = user,
        space = 8
            + (4 + 64)   // sender max 64
            + (4 + 64)   // key max 64
            + (4 + 256)  // value max 256
            + 1,         // bump
        seeds = [b"data_account", user.key().as_ref()],              // seed for deterministic PDA
        bump
    )]
    pub data_account: Account<'info, DataAccount>,

    #[account(mut)]
    pub user: Signer<'info>,
    pub system_program: Program<'info, System>,
}

#[derive(Accounts)]
pub struct UpdateData<'info> {
    #[account(mut)]
    pub user: Signer<'info>,

    // PDA: one account per user, same seeds as Initialize
    #[account(
        mut,
        seeds = [b"data_account", user.key().as_ref()],
        bump,
    )]
    pub data_account: Account<'info, DataAccount>,
}

// just use to have a complex event type ?
#[derive(AnchorSerialize, AnchorDeserialize, Clone, Debug, PartialEq)]
pub struct UserData {
    pub key: String,
    pub value: String,
}

#[derive(AnchorSerialize, AnchorDeserialize, Clone, Debug, PartialEq)]
pub struct ForwarderReport {
    pub account_hash: Vec<u8>,
    pub payload: Vec<u8>,
}

#[event]
pub struct DynamicEvent {
    pub key: String,
    pub user_data: UserData,
    pub sender: String,
    pub metadata: Vec<u8>,
    pub metadata_array: Vec<Vec<u8>>,
}

#[event]
pub struct AccessLogged {
    pub caller: Pubkey,
    pub message: String,
}

#[event]
pub struct NoFields {}

#[error_code]
pub enum DataError {
    #[msg("data not found")]
    DataNotFound = 0,
}

#[derive(AnchorSerialize, AnchorDeserialize, Clone, Debug)]
pub struct UpdateReserves {
    pub total_minted: u64,
    pub total_reserve: u64,
}

// empty contexts
#[derive(Accounts)]
pub struct GetReserves {}
#[derive(Accounts)]
pub struct GetMultipleReserves {}
#[derive(Accounts)]
pub struct GetTupleReserves {}

#[derive(Accounts)]
pub struct HandleForwarderReport {}

#[derive(Accounts)]
pub struct LogAccess<'info> {
    pub user: Signer<'info>,
}

#[derive(Accounts)]
pub struct OnReport<'info> {
    #[account(mut)]
    pub user: Signer<'info>,

    #[account(
        mut,
        seeds = [b"data_account", user.key().as_ref()],
        bump,
    )]
    pub data_account: Account<'info, DataAccount>,

    pub system_program: Program<'info, System>,
}
