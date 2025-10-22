use anchor_lang::prelude::*;

declare_id!("2GvhVcTPPkHbGduj6efNowFoWBQjE77Xab1uBKCYJvNN");

#[program]
pub mod my_project {
    use super::*;

    pub fn initialize(ctx: Context<Initialize>, input: String) -> Result<()> {
        msg!("Greetings from: {:?}", ctx.program_id);
        ctx.accounts.data_account.data = input;
        Ok(())
    }

    pub fn get_input_data(_ctx: Context<GetInputData>, input: String) -> Result<()> {
        msg!("Received input: {}", input);
        Ok(())
    }

    // pub fn get_input_data_from_account(
    //     ctx: Context<GetInputDataFromAccount>,
    //     input: String, // just to create proper binding
    // ) -> Result<()> {
    //     let data_account = &ctx.accounts.data_account;
    //     msg!("Data from PDA: {} {}", data_account.data, input);
    //     Ok(())
    // }

    pub fn update_data(ctx: Context<UpdateData>, new_data: String) -> Result<()> {
        require!(new_data.len() <= 64, ErrorCode::DataTooLong);
        let response = new_data.to_string();
        let data_account = &mut ctx.accounts.data_account;
        data_account.data = new_data;
        // Emit the event
        emit!(DataUpdated {
            sender: ctx.accounts.data_account.key(),
            value: response,
        });
        Ok(())
        // Ok(response)
    }

    pub fn log_access(ctx: Context<LogAccess>, message: String) -> Result<()> {
        emit!(AccessLogged {
            caller: ctx.accounts.user.key(),
            message,
        });
        Ok(())
    }

    // pub fn update_data_with_typed_return(
    //     ctx: Context<UpdateData>,
    //     new_data: String,
    // ) -> Result<UpdateResponse> {
    //     require!(new_data.len() <= 64, ErrorCode::DataTooLong);
    //     let response = new_data.to_string();
    //     let data_account = &mut ctx.accounts.data_account;
    //     data_account.data = new_data;
    //     Ok(UpdateResponse { data: response })
    // }
}

#[account]
pub struct DataAccount {
    pub data: String,
}

#[event]
pub struct DataUpdated {
    pub sender: Pubkey,
    pub value: String,
}

#[derive(Accounts)]
pub struct Initialize<'info> {
    #[account(
        init,
        payer = user,
        space = 8 + 4 + 64,       // 8 for discriminator + string space
        seeds = [b"test"],              // seed for deterministic PDA
        bump
    )]
    pub data_account: Account<'info, DataAccount>,

    #[account(mut)]
    pub user: Signer<'info>,
    pub system_program: Program<'info, System>,
}

#[derive(Accounts)]
pub struct GetInputData {}

// #[derive(Accounts)]
// pub struct GetInputDataFromAccount<'info> {
//     #[account(seeds = [b"test"], bump)]
//     pub data_account: Account<'info, DataAccount>,
// }

#[derive(Accounts)]
pub struct UpdateData<'info> {
    #[account(mut, seeds = [b"test"], bump)]
    pub data_account: Account<'info, DataAccount>,
    // Optional (recommended) authority check:
    // #[account(mut)]
    // pub user: Signer<'info>,
    // Add constraint tying user to an authority field if you add one later.
}

#[error_code]
pub enum ErrorCode {
    #[msg("Data too long")]
    DataTooLong = 0,
}

#[derive(Debug, Default, PartialEq, Eq, Clone, AnchorDeserialize, AnchorSerialize)]
pub struct UpdateResponse {
    pub data: String,
}

// Define your event
#[event]
pub struct AccessLogged {
    pub caller: Pubkey,
    pub message: String,
}

#[derive(Accounts)]
pub struct LogAccess<'info> {
    /// The caller — whoever invokes the instruction.
    pub user: Signer<'info>,
}
